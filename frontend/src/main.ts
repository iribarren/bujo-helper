import './style.css';

import {
    Cancel,
    ChooseFolder,
    DefaultOutputDir,
    ImportCSV,
    MediaTypes,
    OpenFolder,
    Start,
} from '../wailsjs/go/main/App';
import {main} from '../wailsjs/go/models';
import {EventsOn} from '../wailsjs/runtime/runtime';

// Payload of the "job:item" event (Go: ItemEvent).
interface ItemEvent {
    index: number;
    title: string;
    creator: string;
    status: 'ok' | 'failed' | 'cancelled';
    path: string;
    source: string;
    matched: string;
    error: string;
}

// Payload of the "job:done" event (Go: DoneEvent).
interface DoneEvent {
    ok: number;
    failed: number;
    cancelled: number;
    error?: string;
}

const STATUS_ICONS: Record<string, string> = {
    pending: '⏳',
    ok: '✔',
    failed: '✖',
    cancelled: '⊘',
};

function byId<T extends HTMLElement>(id: string): T {
    const node = document.getElementById(id);
    if (!node) throw new Error(`missing #${id}`);
    return node as T;
}

const ui = {
    mediaType: byId<HTMLSelectElement>('media-type'),
    banner: byId<HTMLDivElement>('banner'),
    bannerText: byId<HTMLSpanElement>('banner-text'),
    bannerClose: byId<HTMLButtonElement>('banner-close'),
    importBtn: byId<HTMLButtonElement>('import'),
    count: byId<HTMLSpanElement>('count'),
    items: byId<HTMLTextAreaElement>('items'),
    outputDir: byId<HTMLInputElement>('output-dir'),
    browse: byId<HTMLButtonElement>('browse'),
    country: byId<HTMLInputElement>('country'),
    start: byId<HTMLButtonElement>('start'),
    cancel: byId<HTMLButtonElement>('cancel'),
    progressBar: byId<HTMLDivElement>('progress-bar'),
    progressText: byId<HTMLSpanElement>('progress-text'),
    results: byId<HTMLUListElement>('results'),
    summary: byId<HTMLSpanElement>('summary'),
    openFolder: byId<HTMLButtonElement>('open-folder'),
};

let mediaTypes: main.MediaTypeInfo[] = [];
let running = false;
let total = 0;
let finished = 0;
// The folder of the current or last job, for "Open folder".
let jobFolder = '';
let rows = new Map<number, HTMLLIElement>();

// Events can arrive before Start's promise resolves and the rows exist;
// they are held here and applied once the rows are rendered.
let awaitingStart = false;
let earlyItems: ItemEvent[] = [];
let earlyDone: DoneEvent | null = null;

// --- helpers ---------------------------------------------------------------

function el<K extends keyof HTMLElementTagNameMap>(tag: K, className = '', text = ''): HTMLElementTagNameMap[K] {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text) node.textContent = text;
    return node;
}

function errorText(err: unknown): string {
    if (err instanceof Error) return err.message;
    return String(err);
}

function showError(message: string) {
    ui.bannerText.textContent = message;
    ui.banner.hidden = false;
}

function hideError() {
    ui.banner.hidden = true;
}

function baseName(path: string): string {
    return path.split(/[\\/]/).pop() || path;
}

// countItems mirrors batch.ParseLines: one item per non-blank line that does
// not start with '#' and has a non-empty title (the part before the last comma).
function countItems(text: string): number {
    let n = 0;
    for (const raw of text.split('\n')) {
        const line = raw.trim();
        if (line === '' || line.startsWith('#')) continue;
        const comma = line.lastIndexOf(',');
        const title = comma >= 0 ? line.slice(0, comma).trim() : line;
        if (title !== '') n++;
    }
    return n;
}

function plural(n: number, word: string): string {
    return `${n} ${word}${n === 1 ? '' : 's'}`;
}

// --- rendering -------------------------------------------------------------

function updateCount() {
    ui.count.textContent = plural(countItems(ui.items.value), 'item');
}

function updatePlaceholder() {
    const type = mediaTypes.find((t) => t.id === ui.mediaType.value);
    const creator = type?.creatorLabel || 'Creator';
    ui.items.placeholder = `Title, ${creator}\nOne per line, e.g.\nDune, Frank Herbert`;
}

function updateProgress() {
    const pct = total > 0 ? (finished / total) * 100 : 0;
    ui.progressBar.style.width = `${pct}%`;
    ui.progressText.textContent = `${finished} / ${total}`;
}

function setRunning(on: boolean) {
    running = on;
    for (const control of [ui.mediaType, ui.importBtn, ui.items, ui.browse, ui.country, ui.start]) {
        control.disabled = on;
    }
    ui.start.hidden = on;
    ui.cancel.hidden = !on;
    ui.cancel.disabled = false;
}

function renderRows(views: main.ItemView[]) {
    ui.results.replaceChildren();
    rows = new Map();
    for (const v of views) {
        const row = el('li', 'result pending');
        const icon = el('span', 'status', STATUS_ICONS.pending);
        icon.setAttribute('aria-label', 'pending');
        const body = el('div', 'result-body');
        body.append(
            el('div', 'result-title', v.creator ? `${v.title} — ${v.creator}` : v.title),
            el('div', 'result-detail', 'Waiting…'),
        );
        row.append(icon, body);
        ui.results.append(row);
        rows.set(v.index, row);
    }
}

function applyResult(ev: ItemEvent) {
    const row = rows.get(ev.index);
    if (!row) return;
    row.className = `result ${ev.status}`;
    const icon = row.querySelector('.status')!;
    icon.textContent = STATUS_ICONS[ev.status] ?? '?';
    icon.setAttribute('aria-label', ev.status);
    const detail = row.querySelector<HTMLElement>('.result-detail')!;
    if (ev.status === 'ok') {
        detail.textContent = `${baseName(ev.path)} · ${ev.source}`;
        detail.title = ev.matched ? `Matched: ${ev.matched}\n${ev.path}` : ev.path;
    } else {
        detail.textContent = ev.error || ev.status;
        detail.title = '';
    }
    finished++;
    updateProgress();
    ui.openFolder.hidden = false;
}

function finishJob(done: DoneEvent) {
    setRunning(false);
    const parts = [`${done.ok} downloaded`, `${done.failed} failed`];
    if (done.cancelled > 0) parts.push(`${done.cancelled} cancelled`);
    ui.summary.textContent = parts.join(', ');
    ui.openFolder.hidden = false;
    if (done.error) showError(done.error);
}

// --- actions ---------------------------------------------------------------

async function startDownload() {
    if (running) return;
    hideError();
    const country = ui.country.value.trim().toLowerCase();
    if (!/^[a-z]{2}$/.test(country)) {
        showError('Country must be a two-letter code, e.g. es, us, gb.');
        ui.country.focus();
        return;
    }
    if (countItems(ui.items.value) === 0) {
        showError('Add at least one "Title, Creator" line or import a CSV file.');
        ui.items.focus();
        return;
    }

    setRunning(true);
    awaitingStart = true;
    earlyItems = [];
    earlyDone = null;
    ui.summary.textContent = '';
    ui.openFolder.hidden = true;
    jobFolder = ui.outputDir.value;
    try {
        const views = await Start(new main.StartRequest({
            mediaType: ui.mediaType.value,
            text: ui.items.value,
            outputDir: jobFolder,
            country,
        }));
        total = views.length;
        finished = 0;
        renderRows(views);
        updateProgress();
    } catch (err) {
        awaitingStart = false;
        setRunning(false);
        showError(errorText(err));
        return;
    }
    awaitingStart = false;
    earlyItems.forEach(applyResult);
    earlyItems = [];
    if (earlyDone) finishJob(earlyDone);
    earlyDone = null;
}

async function cancelDownload() {
    ui.cancel.disabled = true;
    try {
        await Cancel();
    } catch (err) {
        showError(errorText(err));
        ui.cancel.disabled = false;
    }
}

async function importCSV() {
    hideError();
    try {
        const text = await ImportCSV();
        if (text === '') return; // dialog cancelled
        ui.items.value = text;
        updateCount();
    } catch (err) {
        showError(`Could not import CSV: ${errorText(err)}`);
    }
}

async function browseFolder() {
    try {
        const dir = await ChooseFolder(ui.outputDir.value);
        if (dir) ui.outputDir.value = dir;
    } catch (err) {
        showError(errorText(err));
    }
}

async function openFolder() {
    try {
        await OpenFolder(jobFolder || ui.outputDir.value);
    } catch (err) {
        showError(errorText(err));
    }
}

// --- wiring ----------------------------------------------------------------

EventsOn('job:item', (ev: ItemEvent) => {
    if (awaitingStart) {
        earlyItems.push(ev);
        return;
    }
    applyResult(ev);
});

EventsOn('job:done', (done: DoneEvent) => {
    if (awaitingStart) {
        earlyDone = done;
        return;
    }
    finishJob(done);
});

ui.items.addEventListener('input', updateCount);
ui.mediaType.addEventListener('change', updatePlaceholder);
ui.importBtn.addEventListener('click', importCSV);
ui.browse.addEventListener('click', browseFolder);
ui.start.addEventListener('click', startDownload);
ui.cancel.addEventListener('click', cancelDownload);
ui.openFolder.addEventListener('click', openFolder);
ui.bannerClose.addEventListener('click', hideError);
document.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
        e.preventDefault();
        startDownload();
    }
});

async function init() {
    try {
        mediaTypes = await MediaTypes();
        ui.mediaType.replaceChildren(...mediaTypes.map((t) => {
            const option = el('option', '', t.label);
            option.value = t.id;
            return option;
        }));
        ui.outputDir.value = await DefaultOutputDir();
    } catch (err) {
        showError(errorText(err));
    }
    updatePlaceholder();
    updateCount();
    updateProgress();
    ui.items.focus();
}

init();

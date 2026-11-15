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
import {initPrint} from './print';
import {initSettings} from './settings';

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

// Payload of the "ratelimit:wait" event (Go: RateLimitEvent).
interface RateLimitEvent {
    host: string;
    seconds: number;
}

// An example line for the items placeholder, per media type id.
const EXAMPLES: Record<string, string> = {
    books: 'Dune, Frank Herbert',
    movies: 'Dune, 2021',
};

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
    settingsOpen: byId<HTMLButtonElement>('settings-open'),
    movieTip: byId<HTMLParagraphElement>('movie-tip'),
    movieTipSettings: byId<HTMLButtonElement>('movie-tip-settings'),
    rateLimit: byId<HTMLParagraphElement>('rate-limit'),
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

// The countdown timer of the rate-limit status line, if one is shown.
let rateLimitTimer: number | undefined;

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

// creatorLabel is the creator column of the selected media type, e.g.
// "Author" or "Year".
function creatorLabel(): string {
    const type = mediaTypes.find((t) => t.id === ui.mediaType.value);
    return type?.creatorLabel || 'Creator';
}

function updatePlaceholder() {
    const example = EXAMPLES[ui.mediaType.value];
    const first = `Title, ${creatorLabel()}`;
    ui.items.placeholder = example ? `${first}\nOne per line, e.g.\n${example}` : `${first}\nOne per line`;
}

// updateMovieTip suggests a TMDB key while movies are selected and none is
// saved.
function updateMovieTip() {
    ui.movieTip.hidden = ui.mediaType.value !== 'movies' || settingsDialog.hasTMDBKey();
}

function updateMediaType() {
    updatePlaceholder();
    updateMovieTip();
}

function hideRateLimit() {
    window.clearInterval(rateLimitTimer);
    rateLimitTimer = undefined;
    ui.rateLimit.hidden = true;
    ui.rateLimit.textContent = '';
}

// showRateLimit counts down a rate-limit wait once per second. A newer wait
// replaces the countdown.
function showRateLimit(ev: RateLimitEvent) {
    hideRateLimit();
    let left = Math.ceil(ev.seconds);
    if (!(left > 0)) return;
    const render = () => {
        ui.rateLimit.textContent = `Waiting ${left}s — rate limit at ${ev.host}`;
    };
    render();
    ui.rateLimit.hidden = false;
    rateLimitTimer = window.setInterval(() => {
        left--;
        if (left <= 0) {
            hideRateLimit();
            return;
        }
        render();
    }, 1000);
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
    hideRateLimit();
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
        showError(`Add at least one "Title, ${creatorLabel()}" line or import a CSV file.`);
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

const settingsDialog = initSettings({onChange: updateMovieTip});

EventsOn('job:item', (ev: ItemEvent) => {
    if (awaitingStart) {
        earlyItems.push(ev);
        return;
    }
    applyResult(ev);
});

EventsOn('ratelimit:wait', showRateLimit);

EventsOn('job:done', (done: DoneEvent) => {
    if (awaitingStart) {
        earlyDone = done;
        return;
    }
    finishJob(done);
});

ui.items.addEventListener('input', updateCount);
ui.mediaType.addEventListener('change', updateMediaType);
ui.settingsOpen.addEventListener('click', () => settingsDialog.open());
ui.movieTipSettings.addEventListener('click', () => settingsDialog.open());
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

// --- tabs ------------------------------------------------------------------

// outputFolder is the download output folder; the print sheet starts there.
export function outputFolder(): string {
    return ui.outputDir.value;
}

const printSheet = initPrint({outputFolder, showError, hideError});
const appRoot = byId<HTMLDivElement>('app');
const tabs = [
    {name: 'download', tab: byId<HTMLButtonElement>('tab-download'), panel: byId<HTMLDivElement>('panel-download')},
    {name: 'print', tab: byId<HTMLButtonElement>('tab-print'), panel: byId<HTMLDivElement>('panel-print')},
];

function selectTab(index: number, focus = false) {
    tabs.forEach(({tab, panel}, i) => {
        const on = i === index;
        tab.setAttribute('aria-selected', String(on));
        tab.tabIndex = on ? 0 : -1;
        panel.hidden = !on;
    });
    if (focus) tabs[index].tab.focus();
    appRoot.dataset.tab = tabs[index].name;
    if (tabs[index].name === 'print') printSheet.activate();
}

tabs.forEach(({tab}, i) => {
    tab.addEventListener('click', () => selectTab(i));
    tab.addEventListener('keydown', (e) => {
        const next = {
            ArrowRight: (i + 1) % tabs.length,
            ArrowLeft: (i - 1 + tabs.length) % tabs.length,
            Home: 0,
            End: tabs.length - 1,
        }[e.key];
        if (next === undefined) return;
        e.preventDefault();
        selectTab(next, true);
    });
});

// Ctrl+Enter starts a download; it does nothing while the print sheet or
// the Settings dialog is shown. This capture listener runs before the
// document listener above.
window.addEventListener('keydown', (e) => {
    const blocked = appRoot.dataset.tab === 'print' || settingsDialog.isOpen();
    if (blocked && e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
        e.stopPropagation();
    }
}, true);

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
    updateMediaType();
    updateCount();
    updateProgress();
    ui.items.focus();
}

init();

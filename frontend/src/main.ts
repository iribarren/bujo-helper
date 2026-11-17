import './style.css';

import {
    Cancel,
    ChooseFolder,
    DefaultOutputDir,
    GetSettings,
    ImportCSV,
    MediaTypes,
    OpenFolder,
    SaveSampleCSV,
    Start,
} from '../wailsjs/go/main/App';
import {main} from '../wailsjs/go/models';
import {EventsOn} from '../wailsjs/runtime/runtime';
import {applyDom, has, label, onLangChange, t, tn, type MessageKey} from './i18n';
import {useLanguage} from './language';
import {initPrint, type PrintSheet} from './print';
import {initSettings, type SettingsDialog} from './settings';

// Payload of the "job:item" event (Go: ItemEvent).
interface ItemEvent {
    index: number;
    title: string;
    creator: string;
    status: 'ok' | 'failed' | 'cancelled';
    path: string;
    source: string;
    matched: string;
    // Why the item failed or was cancelled, for the UI to translate:
    // "not_found", "cancelled", "empty_title", "no_providers", or "" when
    // only the English error says.
    code: '' | 'not_found' | 'cancelled' | 'empty_title' | 'no_providers';
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
    tv: 'Breaking Bad, 2008',
    albums: 'Abbey Road, The Beatles',
    games: 'Hollow Knight, 2017',
    artworks: 'Wheat Field with Cypresses, Vincent van Gogh',
    places: 'Eiffel Tower, France',
};

// The tip shown above the list for media types whose best source needs a
// key, while none is saved: what to add, why (message keys), and whether it
// is saved.
interface KeyTip {
    what: MessageKey;
    why: MessageKey;
    saved(): boolean;
}

const TMDB_TIP: KeyTip = {
    what: 'tip.tmdbWhat',
    why: 'tip.tmdbWhy',
    saved: () => settingsDialog?.hasTMDBKey() ?? false,
};

const KEY_TIPS: Record<string, KeyTip> = {
    movies: TMDB_TIP,
    tv: TMDB_TIP,
    games: {
        what: 'tip.igdbWhat',
        why: 'tip.igdbWhy',
        saved: () => settingsDialog?.hasIGDBCredentials() ?? false,
    },
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
    keyTip: byId<HTMLParagraphElement>('key-tip'),
    keyTipWhat: byId<HTMLSpanElement>('key-tip-what'),
    keyTipWhy: byId<HTMLSpanElement>('key-tip-why'),
    keyTipSettings: byId<HTMLButtonElement>('key-tip-settings'),
    rateLimit: byId<HTMLParagraphElement>('rate-limit'),
    banner: byId<HTMLDivElement>('banner'),
    bannerText: byId<HTMLSpanElement>('banner-text'),
    bannerClose: byId<HTMLButtonElement>('banner-close'),
    importBtn: byId<HTMLButtonElement>('import'),
    sampleCsv: byId<HTMLButtonElement>('sample-csv'),
    sampleStatus: byId<HTMLSpanElement>('sample-status'),
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
// The Settings dialog and the print sheet, set up once the language is known.
let settingsDialog: SettingsDialog | undefined;
let printSheet: PrintSheet | undefined;
let running = false;
let total = 0;
let finished = 0;
// The folder of the current or last job, for "Open folder".
let jobFolder = '';
let rows = new Map<number, HTMLLIElement>();
// The result of each finished row, to show it again in another language.
let results = new Map<number, ItemEvent>();
// The summary of the last job, to show it again in another language.
let lastDone: DoneEvent | null = null;

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

// --- rendering -------------------------------------------------------------

function updateCount() {
    ui.count.textContent = tn('download.items', countItems(ui.items.value));
}

function currentMediaType(): main.MediaTypeInfo | undefined {
    return mediaTypes.find((type) => type.id === ui.mediaType.value);
}

// creatorLabel is the creator column of the selected media type, e.g.
// "Author" or "Year".
function creatorLabel(): string {
    const type = currentMediaType();
    return type ? label('creator', type.id, type.creatorLabel || t('download.creator')) : t('download.creator');
}

function updatePlaceholder() {
    const example = EXAMPLES[ui.mediaType.value];
    const creator = creatorLabel();
    ui.items.placeholder = example
        ? t('download.placeholderExample', {creator, example})
        : t('download.placeholder', {creator});
}

// updateKeyTip suggests a key while a media type whose best source needs
// one is selected and none is saved: TMDB for movies and TV series, IGDB
// for games.
function updateKeyTip() {
    const tip = KEY_TIPS[ui.mediaType.value];
    ui.keyTip.hidden = !tip || tip.saved();
    if (tip) {
        ui.keyTipWhat.textContent = t(tip.what);
        ui.keyTipWhy.textContent = t(tip.why);
    }
}

// updateSampleButton names the selected media type on the sample CSV
// button, e.g. "Sample books CSV".
function updateSampleButton() {
    const type = currentMediaType();
    const button = `sample.button.${type?.id}`;
    const title = `sample.title.${type?.id}`;
    if (type && has(button) && has(title)) {
        ui.sampleCsv.textContent = t(button);
        ui.sampleCsv.title = t(title);
    } else {
        ui.sampleCsv.textContent = type
            ? t('sample.buttonFor', {media: type.label.toLowerCase()})
            : t('sample.button');
        ui.sampleCsv.title = t('sample.title', {creator: creatorLabel()});
    }
}

function updateMediaType() {
    updatePlaceholder();
    updateKeyTip();
    updateSampleButton();
    hideSampleStatus();
}

// fillMediaTypes lists the media types in the select, keeping the selection.
function fillMediaTypes() {
    const kept = ui.mediaType.value;
    ui.mediaType.replaceChildren(...mediaTypes.map((type) => {
        const option = el('option', '', label('media', type.id, type.label));
        option.value = type.id;
        return option;
    }));
    if (kept) ui.mediaType.value = kept;
}

// rateLimitDeadline is when the currently shown wait ends (ms since epoch).
let rateLimitDeadline = 0;

function hideRateLimit() {
    window.clearInterval(rateLimitTimer);
    rateLimitTimer = undefined;
    rateLimitDeadline = 0;
    ui.rateLimit.hidden = true;
    ui.rateLimit.textContent = '';
}

// showRateLimit counts down a rate-limit wait once per second. Concurrent
// downloads can report overlapping waits, so a wait that ends before the one
// already shown is ignored: the countdown always shows the latest deadline.
function showRateLimit(ev: RateLimitEvent) {
    const seconds = Math.ceil(ev.seconds);
    if (!(seconds > 0)) return;
    const deadline = Date.now() + seconds * 1000;
    if (deadline <= rateLimitDeadline) return;
    hideRateLimit();
    rateLimitDeadline = deadline;
    const render = () => {
        const left = Math.ceil((rateLimitDeadline - Date.now()) / 1000);
        if (left <= 0) {
            hideRateLimit();
            return;
        }
        ui.rateLimit.textContent = t('download.rateLimit', {seconds: left, host: ev.host});
    };
    render();
    ui.rateLimit.hidden = false;
    rateLimitTimer = window.setInterval(render, 1000);
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

// statusText is the translated name of an item status, for its icon.
function statusText(status: string): string {
    const key = `status.${status}`;
    return has(key) ? t(key) : status;
}

// failureText is why an item failed or was cancelled: the translated reason
// for its code, or Go's English error when there is no code.
function failureText(ev: ItemEvent): string {
    const key = `status.code.${ev.code}`;
    if (ev.code && has(key)) return t(key);
    return ev.error || statusText(ev.status);
}

function renderRows(views: main.ItemView[]) {
    ui.results.replaceChildren();
    rows = new Map();
    results = new Map();
    for (const v of views) {
        const row = el('li', 'result pending');
        const icon = el('span', 'status', STATUS_ICONS.pending);
        icon.setAttribute('aria-label', statusText('pending'));
        const body = el('div', 'result-body');
        body.append(
            el('div', 'result-title', v.creator ? `${v.title} — ${v.creator}` : v.title),
            el('div', 'result-detail', t('download.waiting')),
        );
        row.append(icon, body);
        ui.results.append(row);
        rows.set(v.index, row);
    }
}

// renderResult shows a finished item in its row.
function renderResult(row: HTMLLIElement, ev: ItemEvent) {
    row.className = `result ${ev.status}`;
    const icon = row.querySelector('.status')!;
    icon.textContent = STATUS_ICONS[ev.status] ?? '?';
    icon.setAttribute('aria-label', statusText(ev.status));
    const detail = row.querySelector<HTMLElement>('.result-detail')!;
    if (ev.status === 'ok') {
        detail.textContent = `${baseName(ev.path)} · ${ev.source}`;
        detail.title = ev.matched ? `${t('download.matched', {matched: ev.matched})}\n${ev.path}` : ev.path;
    } else {
        detail.textContent = failureText(ev);
        detail.title = '';
    }
}

function applyResult(ev: ItemEvent) {
    const row = rows.get(ev.index);
    if (!row) return;
    results.set(ev.index, ev);
    renderResult(row, ev);
    finished++;
    updateProgress();
    ui.openFolder.hidden = false;
}

function renderSummary(done: DoneEvent) {
    const downloaded = tn('download.downloaded', done.ok);
    const failed = tn('download.failed', done.failed);
    ui.summary.textContent = done.cancelled > 0
        ? t('download.summaryCancelled', {downloaded, failed, cancelled: tn('download.cancelled', done.cancelled)})
        : t('download.summary', {downloaded, failed});
}

function finishJob(done: DoneEvent) {
    setRunning(false);
    hideRateLimit();
    lastDone = done;
    renderSummary(done);
    ui.openFolder.hidden = false;
    if (done.error) showError(done.error);
}

// --- actions ---------------------------------------------------------------

async function startDownload() {
    if (running) return;
    hideError();
    const country = ui.country.value.trim().toLowerCase();
    if (!/^[a-z]{2}$/.test(country)) {
        showError(t('download.errCountry'));
        ui.country.focus();
        return;
    }
    if (countItems(ui.items.value) === 0) {
        showError(t('download.errNoItems', {creator: creatorLabel()}));
        ui.items.focus();
        return;
    }

    setRunning(true);
    awaitingStart = true;
    earlyItems = [];
    earlyDone = null;
    ui.summary.textContent = '';
    lastDone = null;
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
        showError(t('download.errImport', {error: errorText(err)}));
    }
}

// The timer that clears the "Saved …" note of the sample CSV button, and
// the path it names while it is shown.
let sampleStatusTimer: number | undefined;
let sampleSaved = '';

function hideSampleStatus() {
    window.clearTimeout(sampleStatusTimer);
    sampleSaved = '';
    ui.sampleStatus.textContent = '';
    ui.sampleStatus.title = '';
}

function renderSampleStatus() {
    if (!sampleSaved) return;
    ui.sampleStatus.textContent = t('download.saved', {file: baseName(sampleSaved)});
    ui.sampleStatus.title = sampleSaved;
}

async function saveSampleCSV() {
    // One save at a time: the dialog may not block the window on every
    // platform, and overlapping saves would mix up the "Saved" note.
    if (ui.sampleCsv.disabled) return;
    ui.sampleCsv.disabled = true;
    hideError();
    try {
        const path = await SaveSampleCSV(ui.mediaType.value);
        if (path === '') return; // dialog cancelled
        hideSampleStatus();
        sampleSaved = path;
        renderSampleStatus();
        sampleStatusTimer = window.setTimeout(hideSampleStatus, 5000);
    } catch (err) {
        showError(t('download.errSample', {error: errorText(err)}));
    } finally {
        ui.sampleCsv.disabled = false;
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

// relabel shows the Download tab's dynamic text in the current language,
// keeping the selections, the typed list and the results.
function relabel() {
    fillMediaTypes();
    updatePlaceholder();
    updateKeyTip();
    updateSampleButton();
    renderSampleStatus();
    updateCount();
    for (const [index, row] of rows) {
        const ev = results.get(index);
        if (ev) {
            renderResult(row, ev);
        } else {
            row.querySelector('.status')!.setAttribute('aria-label', statusText('pending'));
            row.querySelector('.result-detail')!.textContent = t('download.waiting');
        }
    }
    if (lastDone) renderSummary(lastDone);
}

// --- wiring ----------------------------------------------------------------

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
ui.settingsOpen.addEventListener('click', () => settingsDialog?.open());
ui.keyTipSettings.addEventListener('click', () => settingsDialog?.open());
ui.importBtn.addEventListener('click', importCSV);
ui.sampleCsv.addEventListener('click', saveSampleCSV);
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
    if (tabs[index].name === 'print') printSheet?.activate();
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
    const blocked = appRoot.dataset.tab === 'print' || (settingsDialog?.isOpen() ?? false);
    if (blocked && e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
        e.stopPropagation();
    }
}, true);

// startLanguage applies the saved language setting, or the system language
// when there is none or it cannot be read, before any text is shown.
async function startLanguage() {
    let setting = '';
    try {
        setting = (await GetSettings()).language || '';
    } catch (err) {
        console.warn(`Could not read the language setting: ${errorText(err)}`);
    }
    await useLanguage(setting);
    applyDom(document);
}

async function init() {
    await startLanguage();
    // On a language change: static markup first, then each module's text.
    onLangChange(() => applyDom(document));
    onLangChange(relabel);
    settingsDialog = initSettings({onChange: updateKeyTip});
    printSheet = initPrint({
        outputFolder,
        showError,
        hideError,
        mediaTypes: () => mediaTypes,
        mediaType: () => ui.mediaType.value,
    });
    try {
        mediaTypes = await MediaTypes();
        fillMediaTypes();
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

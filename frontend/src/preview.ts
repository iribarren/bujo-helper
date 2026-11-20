// A live PDF preview. A render binding (PreviewPage, PreviewPDF) renders
// the current options in Go and answers the URL its PDF is served at (see
// internal/preview); pdf.js draws one page of it onto a <canvas> in a
// page-shaped sheet, with previous and next buttons. Requests are debounced
// and numbered: only the newest one is ever drawn. While a render or a page
// draw is on its way, the sheet is marked busy (dimming the old page) and a
// "Loading preview…" indicator shows over the well.
//
// The legacy build of pdf.js is used: the modern one calls JavaScript
// methods that only the newest engines have (e.g. Map.getOrInsertComputed),
// and the app runs in whatever WebKitGTK, WebView2 or WKWebView the system
// has.

import {getDocument, GlobalWorkerOptions} from 'pdfjs-dist/legacy/build/pdf.mjs';
import workerUrl from 'pdfjs-dist/legacy/build/pdf.worker.min.mjs?url';

import {t, type MessageKey, type Params} from './i18n';

GlobalWorkerOptions.workerSrc = workerUrl;

// DEBOUNCE_MS is how long the options must stay unchanged before a render.
export const DEBOUNCE_MS = 300;

// PreviewInfo is what a render binding answers (main.PreviewInfo).
export interface PreviewInfo {
    url: string;
    pages: number;
}

export interface PreviewRequest {
    // key identifies the options; a request for the options already shown
    // or on their way is ignored.
    key: string;
    render: () => Promise<PreviewInfo>;
}

export interface PreviewElements {
    // The page-shaped box the canvas fills; its --page-ratio is set to the
    // drawn page's width / height. The loading indicator is added next to it
    // in its parent (the well).
    sheet: HTMLElement;
    prev: HTMLButtonElement;
    next: HTMLButtonElement;
    // Shows "Page X of N".
    pageLabel: HTMLElement;
    // Shows an error or why there is nothing to preview.
    message: HTMLElement;
}

export interface PreviewOptions {
    // The debounce delay in milliseconds, DEBOUNCE_MS by default.
    delay?: number;
}

export interface Preview {
    // schedule renders req once the options have not changed for the
    // debounce delay.
    schedule: (req: PreviewRequest) => void;
    // showMessage shows the message key instead of a preview, dropping any
    // pending render.
    showMessage: (key: MessageKey) => void;
    // cancel drops a pending render (e.g. while the preview is hidden),
    // keeping the page shown; scheduling the same options renders them.
    cancel: () => void;
    // relabel shows the page label and message in the current language.
    relabel: () => void;
    // result is the key and page count of the document shown, if any.
    result: () => {key: string; pages: number} | undefined;
}

// The parts of pdf.js used here.
interface PdfPage {
    getViewport: (params: {scale: number}) => {width: number; height: number};
    render: (params: {canvas: HTMLCanvasElement; viewport: {width: number; height: number}}) => {
        promise: Promise<void>;
        cancel: () => void;
    };
}

interface PdfDoc {
    numPages: number;
    getPage: (n: number) => Promise<PdfPage>;
    // pdf.js 6 documents have no destroy of their own: they are freed
    // through the task that loaded them.
    loadingTask: {destroy: () => Promise<void>};
}

interface Shown {
    key: string;
    doc: PdfDoc;
    pages: number;
}

function errorText(err: unknown): string {
    if (err instanceof Error) return err.message;
    return String(err);
}

// loadPdf fetches the PDF at url and opens it with pdf.js. The bytes are
// fetched here rather than by pdf.js, which only streams http(s) URLs and
// the asset server may use a custom scheme (wails://).
async function loadPdf(url: string, isStale: () => boolean): Promise<PdfDoc | undefined> {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const data = new Uint8Array(await res.arrayBuffer());
    if (isStale()) return undefined;
    const task = getDocument({data});
    try {
        const doc = await task.promise as unknown as PdfDoc;
        if (!isStale()) return doc;
    } catch (err) {
        if (!isStale()) throw err;
    }
    // A newer request won while this one loaded: free it.
    void task.destroy();
    return undefined;
}

// free frees a document. Freeing is housekeeping: it never stops the
// preview from showing the next document.
function free(doc: PdfDoc) {
    try {
        doc.loadingTask.destroy().catch(() => {});
    } catch {
        // Nothing to free.
    }
}

// loadingIndicator makes the indicator shown over the well while the
// preview loads: a decorative spinner and a polite status text, which is
// empty at rest so that only its arrival is announced.
function loadingIndicator(): {node: HTMLElement; text: HTMLElement} {
    const node = document.createElement('div');
    node.className = 'preview-loading';
    node.setAttribute('role', 'status');
    node.setAttribute('aria-live', 'polite');
    node.dataset.active = 'false';
    const spinner = document.createElement('span');
    spinner.className = 'preview-spinner';
    spinner.setAttribute('aria-hidden', 'true');
    const text = document.createElement('span');
    text.className = 'preview-loading-text';
    node.append(spinner, text);
    return {node, text};
}

export function initPreview(el: PreviewElements, opts: PreviewOptions = {}): Preview {
    const delay = opts.delay ?? DEBOUNCE_MS;
    const loading = loadingIndicator();
    el.sheet.after(loading.node);
    // seq numbers requests; a result is used only while its number is the
    // latest. drawSeq does the same for drawing pages.
    let seq = 0;
    let drawSeq = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;
    // wanted is the key of the options shown or on their way.
    let wanted = '';
    let shown: Shown | undefined;
    let page = 1;
    let message: {key: MessageKey; params?: Params} | undefined;
    let renderTask: {cancel: () => void} | undefined;
    // The sheet size the shown page was drawn for, to redraw it sharper
    // when the sheet is resized (e.g. its tab was hidden at first).
    let drawnSize = {width: 0, height: 0};
    // The preview is busy while a request is pending or loading
    // (requesting) or the page to be shown is drawing (drawing): the old
    // page stays, dimmed, under the loading indicator until then.
    let requesting = false;
    let drawing = false;
    let busy: boolean | undefined;

    function renderLoading() {
        loading.node.dataset.active = String(Boolean(busy));
        const text = busy ? t('preview.loading') : '';
        // Rewriting the same text could make screen readers repeat it.
        if (loading.text.textContent !== text) loading.text.textContent = text;
    }

    function updateBusy() {
        const now = requesting || drawing;
        if (now === busy) return;
        busy = now;
        el.sheet.setAttribute('aria-busy', String(now));
        renderLoading();
    }

    function renderLabels() {
        el.pageLabel.textContent = shown ? t('preview.pageOf', {page, total: shown.pages}) : '';
        el.prev.disabled = !shown || page <= 1;
        el.next.disabled = !shown || page >= shown.pages;
        el.message.textContent = message ? t(message.key, message.params) : '';
        el.message.hidden = !message;
        renderLoading();
    }

    // forget frees the shown document and stops drawing it; its canvas stays
    // until a new page replaces it.
    function forget() {
        drawSeq++;
        drawing = false;
        renderTask?.cancel();
        renderTask = undefined;
        if (shown) free(shown.doc);
        shown = undefined;
        updateBusy();
    }

    // clear forgets the shown document and its canvas.
    function clear() {
        forget();
        el.sheet.replaceChildren();
    }

    // drop drops pending and in-flight requests.
    function drop() {
        seq++;
        clearTimeout(timer);
        timer = undefined;
    }

    // stop drops requests and the busy state they hold.
    function stop() {
        drop();
        requesting = false;
        updateBusy();
    }

    function sheetSize() {
        const box = el.sheet.getBoundingClientRect();
        return {width: box.width, height: box.height};
    }

    // redrawIfResized redraws the page shown, quietly, when the sheet's size
    // changed (by more than rounding) since it was drawn. Not while a
    // request or a page draw is on its way: a new document is drawn at the
    // sheet's size anyway, and a page being drawn is redrawn once it is
    // drawn, rather than cancelled (which would leave the preview busy
    // until a redraw of it finished).
    function redrawIfResized() {
        if (!shown || requesting || drawing) return;
        const size = sheetSize();
        const changed = Math.abs(size.width - drawnSize.width) > 1 || Math.abs(size.height - drawnSize.height) > 1;
        if (size.width > 0 && changed) void drawPage(page, true);
    }

    // drawPage draws page n of the shown document on a new canvas, which
    // replaces the old one once drawn, so the old page stays until then.
    // The preview stays busy until the newest draw ends; a quiet draw (a
    // sharper redraw of the page shown) does not make it busy.
    async function drawPage(n: number, quiet = false) {
        if (!shown) return;
        const {doc} = shown;
        const mine = ++drawSeq;
        renderTask?.cancel();
        if (!quiet) {
            drawing = true;
            updateBusy();
        }
        const current = () => mine === drawSeq && shown?.doc === doc;
        const done = () => {
            renderTask = undefined;
            drawing = false;
            updateBusy();
        };
        try {
            const pdfPage = await doc.getPage(n);
            if (!current()) return;
            const base = pdfPage.getViewport({scale: 1});
            el.sheet.style.setProperty('--page-ratio', String(base.width / base.height));
            const size = sheetSize();
            const fit = size.width > 0 && size.height > 0
                ? Math.min(size.width / base.width, size.height / base.height)
                : 1;
            const viewport = pdfPage.getViewport({scale: fit * (window.devicePixelRatio || 1)});
            const canvas = document.createElement('canvas');
            canvas.className = 'preview-canvas';
            canvas.width = Math.max(1, Math.floor(viewport.width));
            canvas.height = Math.max(1, Math.floor(viewport.height));
            const task = pdfPage.render({canvas, viewport});
            renderTask = task;
            await task.promise;
            if (!current()) return;
            done();
            drawnSize = size;
            el.sheet.replaceChildren(canvas);
            // A page drawn clears the error of one that could not be.
            if (message) {
                message = undefined;
                renderLabels();
            }
            // The sheet may have been resized while the page drew.
            redrawIfResized();
        } catch (err) {
            // A cancelled draw was replaced by a newer one.
            if (!current() || (err instanceof Error && err.name === 'RenderingCancelledException')) return;
            done();
            // Show the error rather than another page under this page's label.
            el.sheet.replaceChildren();
            message = {key: 'preview.error', params: {error: errorText(err)}};
            renderLabels();
        }
    }

    async function run(req: PreviewRequest, mine: number) {
        const isStale = () => mine !== seq;
        try {
            const info = await req.render();
            if (isStale()) return;
            const doc = await loadPdf(info.url, isStale);
            if (!doc || isStale()) return;
            // The old page stays, dimmed, until the new one is drawn: the
            // request hands the busy state over to the page draw.
            forget();
            requesting = false;
            message = undefined;
            const pages = doc.numPages || info.pages;
            shown = {key: req.key, doc, pages};
            // Stay on the page shown, if the new document has it.
            page = Math.min(Math.max(page, 1), pages);
            renderLabels();
            await drawPage(page);
        } catch (err) {
            if (isStale()) return;
            // Forget the key so the same options can be tried again.
            wanted = '';
            clear();
            requesting = false;
            message = {key: 'preview.error', params: {error: errorText(err)}};
            renderLabels();
            updateBusy();
        }
    }

    function schedule(req: PreviewRequest) {
        if (req.key === wanted) return;
        // Still busy: the indicator stays up from one change to the next.
        drop();
        wanted = req.key;
        const mine = seq;
        requesting = true;
        updateBusy();
        timer = setTimeout(() => {
            timer = undefined;
            void run(req, mine);
        }, delay);
    }

    function showMessage(key: MessageKey) {
        stop();
        clear();
        wanted = '';
        message = {key};
        renderLabels();
    }

    function cancel() {
        stop();
        // A page still drawing finishes unseen.
        drawing = false;
        updateBusy();
        wanted = shown?.key ?? '';
    }

    function step(by: number) {
        if (!shown) return;
        const next = Math.min(Math.max(page + by, 1), shown.pages);
        if (next === page) return;
        page = next;
        renderLabels();
        void drawPage(page);
    }

    el.prev.addEventListener('click', () => step(-1));
    el.next.addEventListener('click', () => step(1));

    // Redraw the page for its new size, once resizing pauses.
    if (typeof ResizeObserver !== 'undefined') {
        let resizeTimer: ReturnType<typeof setTimeout> | undefined;
        new ResizeObserver(() => {
            clearTimeout(resizeTimer);
            resizeTimer = setTimeout(redrawIfResized, 150);
        }).observe(el.sheet);
    }

    updateBusy();
    renderLabels();

    return {
        schedule,
        showMessage,
        cancel,
        relabel: renderLabels,
        result: () => shown && {key: shown.key, pages: shown.pages},
    };
}

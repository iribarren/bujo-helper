// A live PDF preview. A render binding (PreviewPage, PreviewPDF) renders
// the current options in Go and answers the URL its PDF is served at (see
// internal/preview); pdf.js draws one page of it onto a <canvas> in a
// page-shaped sheet, with previous and next buttons. Requests are debounced
// and numbered: only the newest one is ever drawn.
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
    // drawn page's width / height.
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
    destroy: () => Promise<void>;
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

export function initPreview(el: PreviewElements, opts: PreviewOptions = {}): Preview {
    const delay = opts.delay ?? DEBOUNCE_MS;
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

    function setBusy(busy: boolean) {
        el.sheet.setAttribute('aria-busy', String(busy));
    }

    function renderLabels() {
        el.pageLabel.textContent = shown ? t('preview.pageOf', {page, total: shown.pages}) : '';
        el.prev.disabled = !shown || page <= 1;
        el.next.disabled = !shown || page >= shown.pages;
        el.message.textContent = message ? t(message.key, message.params) : '';
        el.message.hidden = !message;
    }

    // forget frees the shown document and stops drawing it; its canvas stays
    // until a new page replaces it.
    function forget() {
        drawSeq++;
        renderTask?.cancel();
        renderTask = undefined;
        if (shown) void shown.doc.destroy();
        shown = undefined;
    }

    // clear forgets the shown document and its canvas.
    function clear() {
        forget();
        el.sheet.replaceChildren();
    }

    // stop drops pending and in-flight requests.
    function stop() {
        seq++;
        clearTimeout(timer);
        timer = undefined;
        setBusy(false);
    }

    function sheetSize() {
        const box = el.sheet.getBoundingClientRect();
        return {width: box.width, height: box.height};
    }

    // drawPage draws page n of the shown document on a new canvas, which
    // replaces the old one once drawn, so the old page stays until then.
    async function drawPage(n: number) {
        if (!shown) return;
        const {doc} = shown;
        const mine = ++drawSeq;
        renderTask?.cancel();
        const current = () => mine === drawSeq && shown?.doc === doc;
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
            renderTask = undefined;
            drawnSize = size;
            el.sheet.replaceChildren(canvas);
            // A page drawn clears the error of one that could not be.
            if (message) {
                message = undefined;
                renderLabels();
            }
        } catch (err) {
            // A cancelled draw was replaced by a newer one.
            if (!current() || (err instanceof Error && err.name === 'RenderingCancelledException')) return;
            renderTask = undefined;
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
            // The old page stays, dimmed, until the new one is drawn.
            forget();
            message = undefined;
            const pages = doc.numPages || info.pages;
            shown = {key: req.key, doc, pages};
            // Stay on the page shown, if the new document has it.
            page = Math.min(Math.max(page, 1), pages);
            renderLabels();
            await drawPage(page);
            if (!isStale()) setBusy(false);
        } catch (err) {
            if (isStale()) return;
            // Forget the key so the same options can be tried again.
            wanted = '';
            clear();
            message = {key: 'preview.error', params: {error: errorText(err)}};
            renderLabels();
            setBusy(false);
        }
    }

    function schedule(req: PreviewRequest) {
        if (req.key === wanted) return;
        stop();
        wanted = req.key;
        const mine = seq;
        setBusy(true);
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
            resizeTimer = setTimeout(() => {
                const size = sheetSize();
                const changed = Math.abs(size.width - drawnSize.width) > 1 || Math.abs(size.height - drawnSize.height) > 1;
                if (shown && size.width > 0 && changed) void drawPage(page);
            }, 150);
        }).observe(el.sheet);
    }

    setBusy(false);
    renderLabels();

    return {
        schedule,
        showMessage,
        cancel,
        relabel: renderLabels,
        result: () => shown && {key: shown.key, pages: shown.pages},
    };
}

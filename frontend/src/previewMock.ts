// A stand-in for pdf.js and the preview fetch, for tests (jsdom has no
// canvas and no asset server). Tests register PDFs by URL with fakePdf;
// fetch answers the URL itself as the bytes, which getDocument reads back
// to find the registered PDF. Use it with:
//
//   vi.mock('pdfjs-dist/legacy/build/pdf.mjs', async () => (await import('./previewMock')).pdfjsMock);
//   vi.mock('pdfjs-dist/legacy/build/pdf.worker.min.mjs?url', () => ({default: '/pdf.worker.min.mjs'}));

import {vi} from 'vitest';

export interface FakePdf {
    // release lets a held PDF finish loading.
    release: () => void;
    // releaseDraw lets the held page draws finish.
    releaseDraw: () => void;
    // holdDraw keeps the page draws started from now on drawing until
    // releaseDraw is called.
    holdDraw: () => void;
    // destroyed reports whether the document's loading task was destroyed
    // (pdf.js 6 documents have no destroy of their own).
    destroyed: () => boolean;
}

interface Entry extends FakePdf {
    pages: number;
    loaded: Promise<void>;
    drawn: Promise<void>;
    failPage?: number;
    destroyFails?: 'throw' | 'reject';
    destroy: () => void;
}

// pdfDocs are the registered PDFs, by URL.
export const pdfDocs = new Map<string, Entry>();
// renders lists the pages drawn, in order.
export const renders: Array<{url: string; page: number}> = [];

export interface FakePdfOptions {
    // hold keeps the PDF loading until release is called.
    hold?: boolean;
    // holdDraw keeps its pages drawing until releaseDraw is called.
    holdDraw?: boolean;
    // failPage is a page whose drawing fails.
    failPage?: number;
    // destroyFails makes destroying its loading task throw ('throw') or
    // reject ('reject'); the task still counts as destroyed.
    destroyFails?: 'throw' | 'reject';
}

// fakePdf registers a PDF of n pages served at url.
export function fakePdf(url: string, pages: number, opts: FakePdfOptions = {}): FakePdf {
    let release = () => {};
    const loaded = opts.hold ? new Promise<void>((done) => { release = done; }) : Promise.resolve();
    // Every hold since the last release: releaseDraw lets them all finish.
    const holds: Array<() => void> = [];
    const releaseDraw = () => holds.splice(0).forEach((done) => done());
    const held = () => new Promise<void>((done) => { holds.push(done); });
    let destroyed = false;
    const entry: Entry = {
        pages,
        loaded,
        drawn: opts.holdDraw ? held() : Promise.resolve(),
        failPage: opts.failPage,
        destroyFails: opts.destroyFails,
        release: () => release(),
        releaseDraw,
        holdDraw: () => { entry.drawn = held(); },
        destroyed: () => destroyed,
        destroy: () => { destroyed = true; },
    };
    pdfDocs.set(url, entry);
    return entry;
}

// resetFakePdfs forgets the registered PDFs and drawn pages, and stubs
// fetch to answer each URL with its own text.
export function resetFakePdfs(): void {
    pdfDocs.clear();
    renders.length = 0;
    vi.stubGlobal('fetch', vi.fn(async (url: string) => ({
        ok: true,
        status: 200,
        arrayBuffer: async () => new TextEncoder().encode(url).buffer,
    })));
}

// getDocument answers a loading task like pdf.js 6's: the document it loads
// has no destroy (pdf.js 6 removed PDFDocumentProxy.destroy); it is freed
// through its loadingTask.
function getDocument(src: {data: Uint8Array}) {
    const url = new TextDecoder().decode(src.data);
    const entry = pdfDocs.get(url);
    const task = {
        promise: Promise.resolve() as Promise<unknown>,
        destroy: (): Promise<void> => {
            entry?.destroy();
            if (entry?.destroyFails === 'throw') throw new Error('cannot destroy');
            if (entry?.destroyFails === 'reject') return Promise.reject(new Error('cannot destroy'));
            return Promise.resolve();
        },
    };
    const doc = entry && {
        numPages: entry.pages,
        getPage: async (n: number) => ({
            getViewport: ({scale}: {scale: number}) => ({width: 595 * scale, height: 842 * scale}),
            render: () => {
                if (n === entry.failPage) return {promise: Promise.reject(new Error(`cannot draw page ${n}`)), cancel: () => {}};
                renders.push({url, page: n});
                // Like pdf.js, a cancelled draw fails with a
                // RenderingCancelledException.
                let cancel = () => {};
                const drawn = entry.drawn;
                const promise = new Promise<void>((done, fail) => {
                    void drawn.then(done);
                    cancel = () => {
                        const err = new Error(`drawing page ${n} cancelled`);
                        err.name = 'RenderingCancelledException';
                        fail(err);
                    };
                });
                return {promise, cancel: () => cancel()};
            },
        }),
        loadingTask: task,
    };
    task.promise = entry && doc
        ? entry.loaded.then(() => doc)
        : Promise.reject(new Error(`no fake PDF at ${url}`));
    return task;
}

export const pdfjsMock = {
    GlobalWorkerOptions: {workerSrc: ''},
    getDocument: vi.fn(getDocument),
};

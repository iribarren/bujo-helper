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
    // destroyed reports whether the document or its loading task was
    // destroyed.
    destroyed: () => boolean;
}

interface Entry extends FakePdf {
    pages: number;
    loaded: Promise<void>;
    drawn: Promise<void>;
    failPage?: number;
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
}

// fakePdf registers a PDF of n pages served at url.
export function fakePdf(url: string, pages: number, opts: FakePdfOptions = {}): FakePdf {
    let release = () => {};
    const loaded = opts.hold ? new Promise<void>((done) => { release = done; }) : Promise.resolve();
    let releaseDraw = () => {};
    const drawn = opts.holdDraw ? new Promise<void>((done) => { releaseDraw = done; }) : Promise.resolve();
    let destroyed = false;
    const entry: Entry = {
        pages,
        loaded,
        drawn,
        failPage: opts.failPage,
        release: () => release(),
        releaseDraw: () => releaseDraw(),
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

function getDocument(src: {data: Uint8Array}) {
    const url = new TextDecoder().decode(src.data);
    const entry = pdfDocs.get(url);
    const doc = entry && {
        numPages: entry.pages,
        getPage: async (n: number) => ({
            getViewport: ({scale}: {scale: number}) => ({width: 595 * scale, height: 842 * scale}),
            render: () => {
                if (n === entry.failPage) return {promise: Promise.reject(new Error(`cannot draw page ${n}`)), cancel: () => {}};
                renders.push({url, page: n});
                return {promise: entry.drawn, cancel: () => {}};
            },
        }),
        destroy: async () => entry.destroy(),
    };
    return {
        promise: entry && doc
            ? entry.loaded.then(() => doc)
            : Promise.reject(new Error(`no fake PDF at ${url}`)),
        destroy: async () => entry?.destroy(),
    };
}

export const pdfjsMock = {
    GlobalWorkerOptions: {workerSrc: ''},
    getDocument: vi.fn(getDocument),
};

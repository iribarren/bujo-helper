import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';

import {en} from './i18n/en';
import {es} from './i18n/es';
import {useLanguage} from './language';
import {fakePdf, pdfDocs, renders, resetFakePdfs} from './previewMock';
import {initPreview, type Preview, type PreviewInfo} from './preview';

vi.mock('pdfjs-dist/legacy/build/pdf.mjs', async () => (await import('./previewMock')).pdfjsMock);
vi.mock('pdfjs-dist/legacy/build/pdf.worker.min.mjs?url', () => ({default: '/pdf.worker.min.mjs'}));

let preview: Preview;

function el<T extends HTMLElement>(id: string): T {
    const node = document.getElementById(id);
    if (!node) throw new Error(`missing #${id}`);
    return node as T;
}

function canvasCount(): number {
    return el('t-sheet').querySelectorAll('canvas').length;
}

// loading is the loading indicator the preview adds to the well.
function loading(): HTMLElement {
    const node = el('t-well').querySelector<HTMLElement>('.preview-loading');
    if (!node) throw new Error('missing .preview-loading');
    return node;
}

// loadingShown reports whether the loading indicator is shown and says so.
function loadingShown(): boolean {
    const shown = loading().dataset.active === 'true';
    expect(loading().textContent).toBe(shown ? 'Loading preview…' : '');
    return shown;
}

// pending is a render binding that answers only when answered.
function pending(url: string, pages: number) {
    let done: (info: PreviewInfo) => void = () => {};
    const render = vi.fn(() => new Promise<PreviewInfo>((resolve) => { done = resolve; }));
    return {render, answer: () => done({url, pages})};
}

// answer is a render binding resolving to a preview of n pages at url.
function answer(url: string, pages: number): () => Promise<PreviewInfo> {
    fakePdf(url, pages);
    return vi.fn(async () => ({url, pages}));
}

// settle lets pending promises run (with fake timers in use).
async function settle() {
    for (let i = 0; i < 20; i++) await Promise.resolve();
}

beforeEach(() => {
    vi.useFakeTimers();
    resetFakePdfs();
    document.body.innerHTML = `
        <button id="t-prev"></button><span id="t-label"></span><button id="t-next"></button>
        <div id="t-well"><div id="t-sheet"></div><p id="t-message" hidden></p></div>`;
    preview = initPreview({
        sheet: el('t-sheet'),
        prev: el('t-prev'),
        next: el('t-next'),
        pageLabel: el('t-label'),
        message: el('t-message'),
    });
});

afterEach(async () => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    await useLanguage('en');
});

describe('schedule', () => {
    it('waits for a pause and makes one render for a burst of changes', async () => {
        const render = answer('/preview?id=1', 1);
        preview.schedule({key: 'a', render});
        await vi.advanceTimersByTimeAsync(200);
        preview.schedule({key: 'b', render});
        await vi.advanceTimersByTimeAsync(200);
        preview.schedule({key: 'c', render});
        expect(render).not.toHaveBeenCalled();
        await vi.advanceTimersByTimeAsync(300);
        expect(render).toHaveBeenCalledTimes(1);
        await settle();
        expect(canvasCount()).toBe(1);
        expect(el('t-label').textContent).toBe('Page 1 of 1');
        expect(preview.result()).toEqual({key: 'c', pages: 1});
    });

    it('does not render the shown options again', async () => {
        const render = answer('/preview?id=1', 1);
        preview.schedule({key: 'a', render});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        preview.schedule({key: 'a', render});
        await vi.advanceTimersByTimeAsync(300);
        expect(render).toHaveBeenCalledTimes(1);
    });

    it('never lets an older, slower render replace a newer one', async () => {
        fakePdf('/preview?id=1', 5);
        let answerOld: (info: PreviewInfo) => void = () => {};
        const old = vi.fn(() => new Promise<PreviewInfo>((done) => { answerOld = done; }));
        preview.schedule({key: 'old', render: old});
        await vi.advanceTimersByTimeAsync(300);
        expect(old).toHaveBeenCalled();

        preview.schedule({key: 'new', render: answer('/preview?id=2', 2)});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(el('t-label').textContent).toBe('Page 1 of 2');

        answerOld({url: '/preview?id=1', pages: 5});
        await settle();
        expect(el('t-label').textContent).toBe('Page 1 of 2');
        expect(preview.result()).toEqual({key: 'new', pages: 2});
        expect(canvasCount()).toBe(1);
    });

    it('drops a stale PDF that finishes loading late, and frees it', async () => {
        const slow = fakePdf('/preview?id=1', 3, {hold: true});
        preview.schedule({key: 'old', render: vi.fn(async () => ({url: '/preview?id=1', pages: 3}))});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        preview.schedule({key: 'new', render: answer('/preview?id=2', 1)});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        slow.release();
        await settle();
        expect(el('t-label').textContent).toBe('Page 1 of 1');
        expect(slow.destroyed()).toBe(true);
    });

    it('frees the previous document when a new one is shown', async () => {
        preview.schedule({key: 'a', render: answer('/preview?id=1', 1)});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        preview.schedule({key: 'b', render: answer('/preview?id=2', 1)});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(pdfDocs.get('/preview?id=1')?.destroyed()).toBe(true);
        expect(pdfDocs.get('/preview?id=2')?.destroyed()).toBe(false);
    });

    it('shows the error when the render fails, and can retry the same options', async () => {
        const failing = vi.fn(async () => { throw new Error('bad paper'); });
        preview.schedule({key: 'a', render: failing});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(el('t-message').hidden).toBe(false);
        expect(el('t-message').textContent).toBe('Could not show the preview: bad paper');
        expect(canvasCount()).toBe(0);
        expect(el('t-label').textContent).toBe('');
        expect(preview.result()).toBeUndefined();

        const render = answer('/preview?id=1', 1);
        preview.schedule({key: 'a', render});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(render).toHaveBeenCalled();
        expect(el('t-message').hidden).toBe(true);
        expect(canvasCount()).toBe(1);
    });

    it('keeps the old page, dimmed, until the next document is drawn', async () => {
        preview.schedule({key: 'a', render: answer('/preview?id=1', 1)});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        const old = el('t-sheet').querySelector('canvas');
        expect(old).not.toBeNull();

        const next = fakePdf('/preview?id=2', 1, {holdDraw: true});
        preview.schedule({key: 'b', render: vi.fn(async () => ({url: '/preview?id=2', pages: 1}))});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(renders.at(-1)).toEqual({url: '/preview?id=2', page: 1});
        expect(el('t-sheet').querySelector('canvas')).toBe(old);
        expect(el('t-sheet').getAttribute('aria-busy')).toBe('true');

        next.releaseDraw();
        await settle();
        expect(canvasCount()).toBe(1);
        expect(el('t-sheet').querySelector('canvas')).not.toBe(old);
        expect(el('t-sheet').getAttribute('aria-busy')).toBe('false');
    });

    it('shows the error when the page cannot be drawn', async () => {
        fakePdf('/preview?id=1', 1, {failPage: 1});
        preview.schedule({key: 'a', render: vi.fn(async () => ({url: '/preview?id=1', pages: 1}))});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(el('t-message').hidden).toBe(false);
        expect(el('t-message').textContent).toBe('Could not show the preview: cannot draw page 1');
        expect(canvasCount()).toBe(0);
        expect(el('t-sheet').getAttribute('aria-busy')).toBe('false');
    });

    it('marks the sheet busy while a render is on its way', async () => {
        preview.schedule({key: 'a', render: answer('/preview?id=1', 1)});
        expect(el('t-sheet').getAttribute('aria-busy')).toBe('true');
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(el('t-sheet').getAttribute('aria-busy')).toBe('false');
    });
});

describe('pages', () => {
    async function show(pages: number) {
        preview.schedule({key: `p${pages}`, render: answer(`/preview?id=${pages}`, pages)});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
    }

    it('steps with previous and next within the document', async () => {
        await show(3);
        const prev = el<HTMLButtonElement>('t-prev');
        const next = el<HTMLButtonElement>('t-next');
        expect(prev.disabled).toBe(true);
        expect(next.disabled).toBe(false);
        next.click();
        await settle();
        expect(el('t-label').textContent).toBe('Page 2 of 3');
        next.click();
        await settle();
        expect(el('t-label').textContent).toBe('Page 3 of 3');
        expect(next.disabled).toBe(true);
        expect(prev.disabled).toBe(false);
        expect(renders.at(-1)).toEqual({url: '/preview?id=3', page: 3});
        prev.click();
        await settle();
        expect(el('t-label').textContent).toBe('Page 2 of 3');
    });

    it('stays on the page when the options change, unless it is gone', async () => {
        await show(3);
        el('t-next').click();
        el('t-next').click();
        await settle();
        await show(4);
        expect(el('t-label').textContent).toBe('Page 3 of 4');
        await show(2);
        expect(el('t-label').textContent).toBe('Page 2 of 2');
    });

    it('shows the error for a page that cannot be drawn, and clears it on the next page drawn', async () => {
        fakePdf('/preview?id=1', 2, {failPage: 2});
        preview.schedule({key: 'a', render: vi.fn(async () => ({url: '/preview?id=1', pages: 2}))});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(canvasCount()).toBe(1);
        el('t-next').click();
        await settle();
        expect(el('t-label').textContent).toBe('Page 2 of 2');
        expect(el('t-message').hidden).toBe(false);
        expect(el('t-message').textContent).toBe('Could not show the preview: cannot draw page 2');
        expect(canvasCount()).toBe(0);
        el('t-prev').click();
        await settle();
        expect(el('t-label').textContent).toBe('Page 1 of 2');
        expect(el('t-message').hidden).toBe(true);
        expect(canvasCount()).toBe(1);
    });

    it('has no page buttons to press before a document is shown', () => {
        expect(el<HTMLButtonElement>('t-prev').disabled).toBe(true);
        expect(el<HTMLButtonElement>('t-next').disabled).toBe(true);
    });

    it('labels the page in the current language', async () => {
        await show(2);
        await useLanguage('es');
        preview.relabel();
        expect(el('t-label').textContent).toBe('Página 1 de 2');
    });
});

describe('messages', () => {
    it('replace the preview without rendering, and cancel a pending one', async () => {
        const render = answer('/preview?id=1', 1);
        preview.schedule({key: 'a', render});
        preview.showMessage('preview.noSelection');
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(render).not.toHaveBeenCalled();
        expect(el('t-message').textContent).toBe('Select covers to preview them.');
        expect(el('t-message').hidden).toBe(false);
        expect(canvasCount()).toBe(0);
        expect(el<HTMLButtonElement>('t-next').disabled).toBe(true);
    });

    it('follow the language', async () => {
        preview.showMessage('preview.noSelection');
        await useLanguage('es');
        preview.relabel();
        expect(el('t-message').textContent).toBe(es['preview.noSelection']);
    });
});

describe('cancel', () => {
    it('drops a pending render, which the same options then render later', async () => {
        const render = answer('/preview?id=1', 1);
        preview.schedule({key: 'a', render});
        preview.cancel();
        await vi.advanceTimersByTimeAsync(300);
        expect(render).not.toHaveBeenCalled();
        expect(el('t-sheet').getAttribute('aria-busy')).toBe('false');
        preview.schedule({key: 'a', render});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(render).toHaveBeenCalledTimes(1);
        expect(canvasCount()).toBe(1);
    });
});

describe('loading indicator', () => {
    it('is a polite status over the well, with a decorative spinner, hidden at rest', () => {
        const node = loading();
        expect(node.parentElement).toBe(el('t-well'));
        expect(node.getAttribute('role')).toBe('status');
        expect(node.getAttribute('aria-live')).toBe('polite');
        expect(node.querySelector('.preview-spinner')?.getAttribute('aria-hidden')).toBe('true');
        expect(loadingShown()).toBe(false);
    });

    it('shows from the option change, on an empty well, until the page is drawn', async () => {
        fakePdf('/preview?id=1', 1);
        const req = pending('/preview?id=1', 1);
        preview.schedule({key: 'a', render: req.render});
        expect(loadingShown()).toBe(true);
        await vi.advanceTimersByTimeAsync(150);
        expect(loadingShown()).toBe(true);
        await vi.advanceTimersByTimeAsync(150);
        expect(req.render).toHaveBeenCalled();
        await settle();
        expect(loadingShown()).toBe(true);
        expect(canvasCount()).toBe(0);
        req.answer();
        await settle();
        expect(canvasCount()).toBe(1);
        expect(loadingShown()).toBe(false);
    });

    it('shows over the old page until the new one is drawn', async () => {
        preview.schedule({key: 'a', render: answer('/preview?id=1', 1)});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(loadingShown()).toBe(false);

        const next = fakePdf('/preview?id=2', 1, {holdDraw: true});
        preview.schedule({key: 'b', render: vi.fn(async () => ({url: '/preview?id=2', pages: 1}))});
        expect(loadingShown()).toBe(true);
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(canvasCount()).toBe(1);
        expect(loadingShown()).toBe(true);
        next.releaseDraw();
        await settle();
        expect(loadingShown()).toBe(false);
    });

    it('gives way to the error when the render fails', async () => {
        preview.schedule({key: 'a', render: vi.fn(async () => { throw new Error('bad paper'); })});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(loadingShown()).toBe(false);
        expect(el('t-message').hidden).toBe(false);
    });

    it('gives way to the error when the page cannot be drawn', async () => {
        fakePdf('/preview?id=1', 1, {failPage: 1});
        preview.schedule({key: 'a', render: vi.fn(async () => ({url: '/preview?id=1', pages: 1}))});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(loadingShown()).toBe(false);
        expect(el('t-message').hidden).toBe(false);
    });

    it('gives way to a message', async () => {
        const req = pending('/preview?id=1', 1);
        preview.schedule({key: 'a', render: req.render});
        await vi.advanceTimersByTimeAsync(300);
        expect(loadingShown()).toBe(true);
        preview.showMessage('preview.noSelection');
        expect(loadingShown()).toBe(false);
        expect(el('t-message').hidden).toBe(false);
    });

    it('hides when the render is cancelled', async () => {
        const req = pending('/preview?id=1', 1);
        preview.schedule({key: 'a', render: req.render});
        await vi.advanceTimersByTimeAsync(300);
        expect(loadingShown()).toBe(true);
        preview.cancel();
        expect(loadingShown()).toBe(false);
    });

    it('follows the language', async () => {
        preview.schedule({key: 'a', render: pending('/preview?id=1', 1).render});
        await useLanguage('es');
        preview.relabel();
        expect(loading().textContent).toBe(es['preview.loading']);
    });

    it('stays, with the old page dimmed, when paging while a new document draws', async () => {
        preview.schedule({key: 'a', render: answer('/preview?id=1', 1)});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        const old = el('t-sheet').querySelector('canvas');

        const next = fakePdf('/preview?id=2', 3, {holdDraw: true});
        preview.schedule({key: 'b', render: vi.fn(async () => ({url: '/preview?id=2', pages: 3}))});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(renders.at(-1)).toEqual({url: '/preview?id=2', page: 1});
        el('t-next').click();
        await settle();
        expect(renders.at(-1)).toEqual({url: '/preview?id=2', page: 2});
        expect(el('t-label').textContent).toBe('Page 2 of 3');
        expect(el('t-sheet').querySelector('canvas')).toBe(old);
        expect(el('t-sheet').getAttribute('aria-busy')).toBe('true');
        expect(loadingShown()).toBe(true);

        next.releaseDraw();
        await settle();
        expect(el('t-sheet').querySelector('canvas')).not.toBe(old);
        expect(el('t-sheet').getAttribute('aria-busy')).toBe('false');
        expect(loadingShown()).toBe(false);
    });

    it('clears when the next document replaces the shown one', async () => {
        // pdf.js 6 documents have no destroy(); freeing the shown one must not
        // break the hand-over (T6: Make pages stuck on the indicator).
        preview.schedule({key: 'a', render: answer('/preview?id=1', 1)});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        preview.schedule({key: 'b', render: answer('/preview?id=2', 2)});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(renders.at(-1)).toEqual({url: '/preview?id=2', page: 1});
        expect(el('t-label').textContent).toBe('Page 1 of 2');
        expect(preview.result()).toEqual({key: 'b', pages: 2});
        expect(el('t-message').hidden).toBe(true);
        expect(el('t-sheet').getAttribute('aria-busy')).toBe('false');
        expect(loadingShown()).toBe(false);
        expect(pdfDocs.get('/preview?id=1')?.destroyed()).toBe(true);
    });
});

describe('resize', () => {
    // The sheet's size, as getBoundingClientRect reports it, and the
    // ResizeObserver callback (jsdom has neither layout nor ResizeObserver).
    let size = {width: 0, height: 0};
    let notify: () => void = () => {};

    function resize(width: number, height: number) {
        size = {width, height};
        notify();
    }

    function canvasSize(): string {
        const canvas = el('t-sheet').querySelector('canvas');
        return canvas ? `${canvas.width}x${canvas.height}` : 'none';
    }

    beforeEach(() => {
        size = {width: 0, height: 0};
        vi.stubGlobal('ResizeObserver', class {
            constructor(callback: () => void) {
                notify = callback;
            }
            observe() {}
            disconnect() {}
        });
        document.body.innerHTML = `
            <button id="t-prev"></button><span id="t-label"></span><button id="t-next"></button>
            <div id="t-well"><div id="t-sheet"></div><p id="t-message" hidden></p></div>`;
        el('t-sheet').getBoundingClientRect = () => ({...size}) as DOMRect;
        preview = initPreview({
            sheet: el('t-sheet'),
            prev: el('t-prev'),
            next: el('t-next'),
            pageLabel: el('t-label'),
            message: el('t-message'),
        });
    });

    it('redraws the page for the new size quietly, once', async () => {
        // Drawn while its tab is hidden (size 0), at the PDF's own size.
        const pdf = fakePdf('/preview?id=1', 1);
        preview.schedule({key: 'a', render: vi.fn(async () => ({url: '/preview?id=1', pages: 1}))});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(canvasSize()).toBe('595x842');

        pdf.holdDraw();
        resize(297.5, 421);
        await vi.advanceTimersByTimeAsync(150);
        await settle();
        expect(renders).toHaveLength(2);
        // A sharper redraw of the page shown is not loading.
        expect(el('t-sheet').getAttribute('aria-busy')).toBe('false');
        expect(loadingShown()).toBe(false);
        pdf.releaseDraw();
        await settle();
        expect(canvasSize()).toBe('297x421');
        expect(loadingShown()).toBe(false);

        // Same size (or a rounding away): no redraw.
        resize(297.9, 421.4);
        await vi.advanceTimersByTimeAsync(150);
        await settle();
        expect(renders).toHaveLength(2);
    });

    it('lets the page being drawn finish, then redraws it for the new size', async () => {
        const pdf = fakePdf('/preview?id=1', 1, {holdDraw: true});
        preview.schedule({key: 'a', render: vi.fn(async () => ({url: '/preview?id=1', pages: 1}))});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(renders).toHaveLength(1);
        expect(loadingShown()).toBe(true);

        resize(297.5, 421);
        await vi.advanceTimersByTimeAsync(150);
        await settle();
        // Not cancelled for a quiet redraw: the indicator waits for it.
        expect(renders).toHaveLength(1);
        expect(loadingShown()).toBe(true);

        pdf.releaseDraw();
        await settle();
        expect(renders).toHaveLength(2);
        expect(canvasSize()).toBe('297x421');
        expect(el('t-sheet').getAttribute('aria-busy')).toBe('false');
        expect(loadingShown()).toBe(false);
    });

    it('does not redraw the old page while new options render', async () => {
        resize(297.5, 421);
        preview.schedule({key: 'a', render: answer('/preview?id=1', 1)});
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(renders).toHaveLength(1);

        // A new paper reshapes the sheet before its PDF arrives.
        fakePdf('/preview?id=2', 1);
        const req = pending('/preview?id=2', 1);
        preview.schedule({key: 'b', render: req.render});
        resize(325, 421);
        await vi.advanceTimersByTimeAsync(300);
        await settle();
        expect(renders).toEqual([{url: '/preview?id=1', page: 1}]);
        expect(loadingShown()).toBe(true);

        req.answer();
        await settle();
        expect(renders).toEqual([{url: '/preview?id=1', page: 1}, {url: '/preview?id=2', page: 1}]);
        expect(loadingShown()).toBe(false);
    });

    it('shows the indicator when paging replaces a quiet redraw, until the page is drawn', async () => {
        const pdf = fakePdf('/preview?id=1', 2);
        preview.schedule({key: 'a', render: vi.fn(async () => ({url: '/preview?id=1', pages: 2}))});
        await vi.advanceTimersByTimeAsync(300);
        await settle();

        pdf.holdDraw();
        resize(297.5, 421);
        await vi.advanceTimersByTimeAsync(150);
        await settle();
        expect(loadingShown()).toBe(false);
        el('t-next').click();
        await settle();
        expect(renders.at(-1)).toEqual({url: '/preview?id=1', page: 2});
        expect(loadingShown()).toBe(true);
        pdf.releaseDraw();
        await settle();
        expect(el('t-label').textContent).toBe('Page 2 of 2');
        expect(loadingShown()).toBe(false);
    });
});

describe('messages exist in English and Spanish', () => {
    const keys = Object.keys(en).filter((k) => k.startsWith('preview.'));
    it('has the preview messages', () => {
        expect(keys).toEqual(expect.arrayContaining(['preview.pageOf', 'preview.prev', 'preview.next', 'preview.error', 'preview.noSelection', 'preview.loading']));
    });
    it.each(keys)('%s is translated', (key) => {
        expect((es as Record<string, string>)[key]).toBeTruthy();
    });
});

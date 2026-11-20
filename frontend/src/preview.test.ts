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

describe('messages exist in English and Spanish', () => {
    const keys = Object.keys(en).filter((k) => k.startsWith('preview.'));
    it('has the preview messages', () => {
        expect(keys).toEqual(expect.arrayContaining(['preview.pageOf', 'preview.prev', 'preview.next', 'preview.error', 'preview.noSelection']));
    });
    it.each(keys)('%s is translated', (key) => {
        expect((es as Record<string, string>)[key]).toBeTruthy();
    });
});

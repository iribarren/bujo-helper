import {describe, expect, it, vi} from 'vitest';

import {LetteringFonts} from '../wailsjs/go/main/App';
import {main} from '../wailsjs/go/models';
import {letteringFonts} from './lettering';

// letteringFonts keeps its answer for the whole module, so these tests live
// in their own file: the cache starts empty here.

vi.mock('../wailsjs/go/main/App', () => ({LetteringFonts: vi.fn()}));

const font = (id: string, url: string) => main.LetteringFontInfo.createFrom({
    id, label: id, group: 'display', hollow: 'stroke', licence: 'SIL OFL 1.1',
    url, licenceUrl: `/fonts/${id}/licence`, default: id === 'go',
});

function faces(): string | undefined {
    return document.getElementById('lettering-font-faces')?.textContent ?? undefined;
}

describe('letteringFonts load failures', () => {
    it('forgets a failed call, adds no @font-face, and asks again next time', async () => {
        vi.mocked(LetteringFonts).mockRejectedValueOnce(new Error('boom'));
        await expect(letteringFonts()).rejects.toThrow('boom');
        expect(document.getElementById('lettering-font-faces')).toBeNull();

        vi.mocked(LetteringFonts).mockResolvedValueOnce([
            font('titan-one', '/fonts/titan-one.ttf'),
            // A malformed id and a missing URL get no rule.
            font('Bad Font"}', '/fonts/bad.ttf'),
            font('no-url', ''),
        ]);
        // The failed call is forgotten in a catch handler, which may run a
        // few microtasks after the caller saw the failure: until then the
        // failure is answered again, without a new call. Ask until it is.
        const fonts = await vi.waitFor(() => letteringFonts());
        expect(LetteringFonts).toHaveBeenCalledTimes(2);
        expect(fonts.map((f) => f.id)).toEqual(['titan-one', 'Bad Font"}', 'no-url']);
        const css = faces() ?? '';
        expect(css.match(/@font-face/g)?.length).toBe(1);
        expect(css).toContain('font-family: "lettering-titan-one"');
        expect(css).not.toContain('Bad Font');
        expect(css).not.toContain('lettering-no-url');

        // The answer is now kept.
        await letteringFonts();
        expect(LetteringFonts).toHaveBeenCalledTimes(2);
    });
});

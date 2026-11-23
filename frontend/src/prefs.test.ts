import {afterEach, describe, expect, it, vi} from 'vitest';

import {prefsSaver, restoreSelect} from './prefs';

// settle lets pending promise callbacks run.
const settle = () => new Promise((resolve) => setTimeout(resolve, 0));

describe('restoreSelect', () => {
    function select(...ids: string[]): HTMLSelectElement {
        const node = document.createElement('select');
        for (const id of ids) {
            const option = document.createElement('option');
            option.value = id;
            node.append(option);
        }
        node.value = ids[0];
        return node;
    }

    it('selects an offered id', () => {
        const node = select('a', 'b');
        expect(restoreSelect(node, 'b')).toBe(true);
        expect(node.value).toBe('b');
    });

    it('keeps the current choice for an id not offered, or none', () => {
        const node = select('a', 'b');
        expect(restoreSelect(node, 'gone')).toBe(false);
        expect(node.value).toBe('a');
        expect(restoreSelect(node, '')).toBe(false);
        expect(restoreSelect(node, undefined)).toBe(false);
        expect(node.value).toBe('a');
    });
});

describe('prefsSaver', () => {
    afterEach(() => vi.restoreAllMocks());

    it('saves nothing until started, and nothing unchanged after', async () => {
        let value = {paper: 'a4'};
        const save = vi.fn(async () => {});
        const saver = prefsSaver('test', () => value, save);
        saver.save();
        await settle();
        expect(save).not.toHaveBeenCalled();
        saver.start();
        saver.save();
        await settle();
        expect(save).not.toHaveBeenCalled();
        value = {paper: 'a5'};
        saver.save();
        await settle();
        expect(save).toHaveBeenCalledExactlyOnceWith({paper: 'a5'});
    });

    it('saves one at a time, the latest last', async () => {
        let value = 1;
        const done: Array<() => void> = [];
        const save = vi.fn(() => new Promise<void>((resolve) => done.push(resolve)));
        const saver = prefsSaver('test', () => value, save);
        saver.start();
        value = 2;
        saver.save();
        value = 3;
        saver.save();
        value = 4;
        saver.save();
        await settle();
        expect(save.mock.calls).toEqual([[2]]);
        done[0]();
        await settle();
        expect(save.mock.calls).toEqual([[2], [4]]);
        done[1]();
        await settle();
        expect(save).toHaveBeenCalledTimes(2);
    });

    it('logs a failed save and tries again on the next change', async () => {
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
        let value = 'a';
        const save = vi.fn(async () => {});
        save.mockRejectedValueOnce(new Error('disk full'));
        const saver = prefsSaver('Print covers', () => value, save);
        saver.start();
        value = 'b';
        saver.save();
        await settle();
        expect(warn).toHaveBeenCalledWith('Could not remember the Print covers options: disk full');
        // The failed value was not saved, so the same value is sent again.
        saver.save();
        await settle();
        expect(save.mock.calls).toEqual([['b'], ['b']]);
    });
});

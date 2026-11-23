import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';

import {SearchQuotes} from '../wailsjs/go/main/App';
import {applyDom, setLang} from './i18n';
import {pageForms, type PageForm} from './pageForms';
import {parseDefaults} from './pages';
import {quoteLines} from './quotesForm';

vi.mock('../wailsjs/go/main/App', async (importOriginal) => {
    const original = await importOriginal<Record<string, unknown>>();
    return {...original, SearchQuotes: vi.fn()};
});

// The quotes kind's defaults as PageKinds sends them (Go: sheet.QuotesKind).
const QUOTES_DEFAULTS = '{"quotes":[],"style":"classic","perPage":6}';

interface Result {
    person: string;
    url: string;
    matches: string[];
    quotes: Array<{text: string; author: string; source: string}>;
}

// result is a SearchQuotes result for person with quotes texts.
function result(person: string, texts: string[], matches: string[] = []): Result {
    return {
        person,
        url: `https://en.wikiquote.org/wiki/${person}`,
        matches,
        quotes: texts.map((text) => ({text, author: person, source: 'wikiquote'})),
    };
}

const EINSTEIN = result('Albert Einstein', ['Imagination is more important than knowledge.', 'Life is like riding a bicycle.'], ['Einstein family']);
const TWAIN = result('Mark Twain', ['Kindness is a language the deaf can hear.']);

// deferred is a promise resolved or rejected by hand, for SearchQuotes.
function deferred<T>() {
    let resolve!: (v: T) => void;
    let reject!: (e: unknown) => void;
    const promise = new Promise<T>((res, rej) => {
        resolve = res;
        reject = rej;
    });
    return {promise, resolve, reject};
}

let box: HTMLDivElement;
let onChange: ReturnType<typeof vi.fn>;
let built: PageForm[] = [];

function quotesForm(defaults = QUOTES_DEFAULTS): PageForm {
    box = document.createElement('div');
    document.body.replaceChildren(box);
    onChange = vi.fn();
    const form = pageForms.quotes(box, 'test-quotes', parseDefaults(defaults), onChange);
    built.push(form);
    return form;
}

function q<T extends HTMLElement>(selector: string): T {
    const node = box.querySelector<T>(selector);
    if (!node) throw new Error(`missing ${selector}`);
    return node;
}

const query = () => q<HTMLInputElement>('#test-quotes-query');
const searchButton = () => q<HTMLButtonElement>('#test-quotes-search');
const status = () => q<HTMLElement>('#test-quotes-status');
const person = () => q<HTMLElement>('#test-quotes-person');
const own = () => q<HTMLTextAreaElement>('#test-quotes-own');
const boxes = () => Array.from(box.querySelectorAll<HTMLInputElement>('#test-quotes-results input[type="checkbox"]'));
const boxLabels = () => boxes().map((b) => b.closest('label')?.textContent);
const matchButtons = () => Array.from(box.querySelectorAll<HTMLButtonElement>('#test-quotes-matches button'));
const pickedItems = () => Array.from(box.querySelectorAll<HTMLElement>('#test-quotes-picked li'));

function radio(group: string, value: string): HTMLButtonElement {
    return q<HTMLButtonElement>(`#test-quotes-${group} [role="radio"][data-value="${value}"]`);
}

function checked(group: string): string {
    return box.querySelector<HTMLElement>(`#test-quotes-${group} [role="radio"][aria-checked="true"]`)?.dataset.value ?? '';
}

function typeQuery(text: string) {
    query().value = text;
    query().dispatchEvent(new Event('input'));
}

function typeOwn(text: string) {
    own().value = text;
    own().dispatchEvent(new Event('input'));
}

function pressEnter() {
    query().dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', bubbles: true, cancelable: true}));
}

// search types text, clicks Search with SearchQuotes answering r and waits
// for the results.
async function search(text: string, r: Result) {
    vi.mocked(SearchQuotes).mockResolvedValueOnce(r as never);
    typeQuery(text);
    searchButton().click();
    await vi.waitFor(() => expect(searchButton().disabled).toBe(false));
}

function tick(i: number) {
    boxes()[i].click();
}

beforeEach(() => {
    vi.mocked(SearchQuotes).mockReset();
});

afterEach(() => {
    for (const form of built) form.dispose?.();
    built = [];
    setLang('en');
});

describe('quote cards form', () => {
    it('has the search, own quotes, Style and Cards per page fields, at the defaults', () => {
        const form = quotesForm();
        expect(box.querySelector(`label[for="${query().id}"]`)?.textContent).toBe('Find quotes by');
        expect(query().placeholder).toBe('Author or personality');
        expect(searchButton().textContent).toBe('Search');
        expect(searchButton().type).toBe('button');
        expect(status().getAttribute('role')).toBe('status');
        expect(status().textContent).toBe('');
        expect(person().hidden).toBe(true);
        expect(q('#test-quotes-results-field').hidden).toBe(true);
        expect(q('#test-quotes-picked-field').hidden).toBe(true);
        expect(own().tagName).toBe('TEXTAREA');
        expect(box.querySelector(`label[for="${own().id}"]`)?.textContent).toBe('Your own quotes: Quote, Author');
        expect(Array.from(q('#test-quotes-style').children, (r) => r.textContent)).toEqual(['Classic', 'Bold', 'Framed']);
        expect(Array.from(q('#test-quotes-perPage').children, (r) => r.textContent)).toEqual(['4', '6', '8']);
        expect(checked('style')).toBe('classic');
        expect(checked('perPage')).toBe('6');
        expect(form.options()).toEqual({quotes: [], style: 'classic', perPage: 6});
    });

    it('starts from the defaults it is given', () => {
        const form = quotesForm('{"quotes":[{"text":"Hi","author":"Me","source":""}],"style":"framed","perPage":8}');
        expect(own().value).toBe('Hi, Me');
        expect(form.options()).toEqual({quotes: [{text: 'Hi', author: 'Me', source: ''}], style: 'framed', perPage: 8});
        expect(quotesForm('{"style":"odd","perPage":5}').options()).toEqual({quotes: [], style: 'classic', perPage: 6});
    });

    it('searches the trimmed query with the button or Enter, and not an empty one', async () => {
        quotesForm();
        vi.mocked(SearchQuotes).mockResolvedValue(EINSTEIN as never);
        typeQuery('   ');
        searchButton().click();
        pressEnter();
        expect(SearchQuotes).not.toHaveBeenCalled();

        typeQuery('  Einstein ');
        searchButton().click();
        expect(SearchQuotes).toHaveBeenLastCalledWith('Einstein');
        await vi.waitFor(() => expect(searchButton().disabled).toBe(false));
        typeQuery('Twain');
        pressEnter();
        expect(SearchQuotes).toHaveBeenLastCalledWith('Twain');
        expect(SearchQuotes).toHaveBeenCalledTimes(2);
        await vi.waitFor(() => expect(searchButton().disabled).toBe(false));
    });

    it('disables Search and says so while searching', async () => {
        quotesForm();
        const pending = deferred<Result>();
        vi.mocked(SearchQuotes).mockReturnValueOnce(pending.promise as never);
        typeQuery('Einstein');
        searchButton().click();
        expect(searchButton().disabled).toBe(true);
        expect(status().textContent).toBe('Searching Wikiquote…');
        pending.resolve(EINSTEIN);
        await vi.waitFor(() => expect(searchButton().disabled).toBe(false));
        expect(status().textContent).toBe('');
    });

    it('shows the person, the quotes as checkboxes and the other matches', async () => {
        const form = quotesForm();
        await search('Einstein', EINSTEIN);
        expect(person().hidden).toBe(false);
        expect(person().textContent).toContain('Albert Einstein · 2 quotes');
        expect(person().textContent).toContain('From Wikiquote');
        expect(person().querySelector('a')).toBeNull();
        expect(boxLabels()).toEqual(EINSTEIN.quotes.map((x) => x.text));
        expect(boxes().every((b) => !b.checked)).toBe(true);
        expect(q('#test-quotes-count').textContent).toBe('0 selected');
        expect(matchButtons().map((b) => b.textContent)).toEqual(['Einstein family']);
        const list = q('#test-quotes-results');
        expect(list.tabIndex).toBe(0);
        expect(list.getAttribute('aria-label')).toBe('Quotes found');
        expect(onChange).not.toHaveBeenCalled();
        expect(form.options().quotes).toEqual([]);
    });

    it('sends the ticked quotes as Wikiquote quotes by the person', async () => {
        const form = quotesForm();
        await search('Einstein', EINSTEIN);
        tick(1);
        expect(onChange).toHaveBeenCalledTimes(1);
        expect(q('#test-quotes-count').textContent).toBe('1 selected');
        expect(form.options().quotes).toEqual([{text: 'Life is like riding a bicycle.', author: 'Albert Einstein', source: 'wikiquote'}]);
        tick(0);
        expect(form.options().quotes).toEqual([
            {text: 'Life is like riding a bicycle.', author: 'Albert Einstein', source: 'wikiquote'},
            {text: 'Imagination is more important than knowledge.', author: 'Albert Einstein', source: 'wikiquote'},
        ]);
        tick(1);
        expect(onChange).toHaveBeenCalledTimes(3);
        expect((form.options().quotes as unknown[]).length).toBe(1);
    });

    it('ticks all or none of the results', async () => {
        const form = quotesForm();
        await search('Einstein', EINSTEIN);
        tick(1);
        q<HTMLButtonElement>('#test-quotes-all').click();
        expect(boxes().every((b) => b.checked)).toBe(true);
        expect(q('#test-quotes-count').textContent).toBe('2 selected');
        // The quote ticked first stays first.
        expect((form.options().quotes as Array<{text: string}>).map((x) => x.text))
            .toEqual(['Life is like riding a bicycle.', 'Imagination is more important than knowledge.']);
        expect(onChange).toHaveBeenCalledTimes(2);
        q<HTMLButtonElement>('#test-quotes-all').click();
        expect(onChange).toHaveBeenCalledTimes(2);
        q<HTMLButtonElement>('#test-quotes-none').click();
        expect(boxes().some((b) => b.checked)).toBe(false);
        expect(form.options().quotes).toEqual([]);
        expect(onChange).toHaveBeenCalledTimes(3);
    });

    it('applies only the newest search response', async () => {
        quotesForm();
        const first = deferred<Result>();
        const second = deferred<Result>();
        vi.mocked(SearchQuotes).mockReturnValueOnce(first.promise as never).mockReturnValueOnce(second.promise as never);
        typeQuery('Einstein');
        pressEnter();
        typeQuery('Twain');
        pressEnter();
        second.resolve(TWAIN);
        await vi.waitFor(() => expect(searchButton().disabled).toBe(false));
        first.resolve(EINSTEIN);
        await Promise.resolve();
        await new Promise((r) => setTimeout(r, 0));
        expect(person().textContent).toContain('Mark Twain');
        expect(boxLabels()).toEqual(['Kindness is a language the deaf can hear.']);
        expect(searchButton().disabled).toBe(false);
    });

    it('ignores a stale failure', async () => {
        quotesForm();
        const first = deferred<Result>();
        vi.mocked(SearchQuotes).mockReturnValueOnce(first.promise as never).mockResolvedValueOnce(TWAIN as never);
        typeQuery('Einstein');
        pressEnter();
        typeQuery('Twain');
        pressEnter();
        await vi.waitFor(() => expect(person().textContent).toContain('Mark Twain'));
        first.reject('network down');
        await new Promise((r) => setTimeout(r, 0));
        expect(status().textContent).toBe('');
    });

    it('shows an error and keeps the previous results', async () => {
        quotesForm();
        await search('Einstein', EINSTEIN);
        vi.mocked(SearchQuotes).mockRejectedValueOnce('No quotes found for "Zzz".');
        typeQuery('Zzz');
        searchButton().click();
        await vi.waitFor(() => expect(status().textContent).toBe('No quotes found for "Zzz".'));
        expect(status().classList.contains('error')).toBe(true);
        expect(searchButton().disabled).toBe(false);
        expect(person().textContent).toContain('Albert Einstein');
        expect(boxes().length).toBe(2);

        vi.mocked(SearchQuotes).mockRejectedValueOnce(new Error('timeout'));
        searchButton().click();
        await vi.waitFor(() => expect(status().textContent).toBe('timeout'));
        await search('Twain', TWAIN);
        expect(status().textContent).toBe('');
        expect(status().classList.contains('error')).toBe(false);
    });

    it('says when the page has no quotes and offers the other matches', async () => {
        quotesForm();
        await search('Einstein', result('Einstein (disambiguation)', [], ['Albert Einstein', 'Einstein family']));
        expect(person().textContent).toContain('No quotes found on Einstein (disambiguation).');
        expect(boxes()).toEqual([]);
        expect(q('#test-quotes-results-field').hidden).toBe(true);
        expect(matchButtons().map((b) => b.textContent)).toEqual(['Albert Einstein', 'Einstein family']);

        vi.mocked(SearchQuotes).mockResolvedValueOnce(EINSTEIN as never);
        matchButtons()[0].click();
        expect(SearchQuotes).toHaveBeenLastCalledWith('Albert Einstein');
        expect(query().value).toBe('Albert Einstein');
        await vi.waitFor(() => expect(person().textContent).toContain('Albert Einstein · 2 quotes'));
        expect(boxes().length).toBe(2);
    });

    it('offers at most five other matches', async () => {
        quotesForm();
        await search('A', result('A', ['A quote long enough.'], ['B', 'C', 'D', 'E', 'F', 'G']));
        expect(matchButtons().map((b) => b.textContent)).toEqual(['B', 'C', 'D', 'E', 'F']);
    });

    it('keeps picks from earlier searches, listed with a remove button', async () => {
        const form = quotesForm();
        await search('Einstein', EINSTEIN);
        tick(0);
        await search('Twain', TWAIN);
        expect(boxLabels()).toEqual(['Kindness is a language the deaf can hear.']);
        tick(0);
        expect(form.options().quotes).toEqual([
            {text: 'Imagination is more important than knowledge.', author: 'Albert Einstein', source: 'wikiquote'},
            {text: 'Kindness is a language the deaf can hear.', author: 'Mark Twain', source: 'wikiquote'},
        ]);
        expect(q('#test-quotes-picked-field').hidden).toBe(false);
        expect(q('#test-quotes-picked-title').textContent).toBe('Picked quotes (2)');
        expect(pickedItems().map((li) => li.querySelector('.picked-text')?.textContent))
            .toEqual(['Imagination is more important than knowledge. — Albert Einstein', 'Kindness is a language the deaf can hear. — Mark Twain']);
        const remove = pickedItems()[1].querySelector<HTMLButtonElement>('button')!;
        expect(remove.getAttribute('aria-label')).toBe('Remove quote: Kindness is a language the deaf can hear.');
        expect(remove.textContent).toBe('×');
        onChange.mockClear();
        remove.click();
        expect(onChange).toHaveBeenCalledTimes(1);
        expect(boxes()[0].checked).toBe(false);
        expect(pickedItems().length).toBe(1);
        pickedItems()[0].querySelector<HTMLButtonElement>('button')!.click();
        expect(form.options().quotes).toEqual([]);
        expect(q('#test-quotes-picked-field').hidden).toBe(true);
    });

    it('shows a quote picked earlier as ticked and does not pick it twice', async () => {
        const form = quotesForm();
        await search('Einstein', EINSTEIN);
        tick(0);
        await search('Twain', TWAIN);
        await search('Einstein', EINSTEIN);
        expect(boxes()[0].checked).toBe(true);
        q<HTMLButtonElement>('#test-quotes-all').click();
        expect((form.options().quotes as unknown[]).length).toBe(2);
        // Same text by someone else is another quote.
        await search('Other', result('Other', ['Imagination is more important than knowledge.']));
        expect(boxes()[0].checked).toBe(false);
        tick(0);
        expect((form.options().quotes as unknown[]).length).toBe(3);
    });

    it('lists a result only once', async () => {
        quotesForm();
        await search('A', result('A', ['Same quote here.', 'Same quote here.']));
        expect(boxLabels()).toEqual(['Same quote here.']);
    });

    it('sends the typed quotes after the picked ones', async () => {
        const form = quotesForm();
        await search('Twain', TWAIN);
        tick(0);
        onChange.mockClear();
        typeOwn('Be yourself, everyone else is taken, Oscar Wilde\n\nNo author here\r\n  Just me ,  \rLast');
        expect(onChange).toHaveBeenCalledTimes(1);
        expect(form.options().quotes).toEqual([
            {text: 'Kindness is a language the deaf can hear.', author: 'Mark Twain', source: 'wikiquote'},
            {text: 'Be yourself, everyone else is taken', author: 'Oscar Wilde', source: ''},
            {text: 'No author here', author: '', source: ''},
            {text: 'Just me', author: '', source: ''},
            {text: 'Last', author: '', source: ''},
        ]);
        typeOwn('Be yourself, everyone else is taken, Oscar Wilde\n\nNo author here\r\n  Just me ,  \rLast\n');
        expect(onChange).toHaveBeenCalledTimes(1);
    });

    it('sends the chosen style and cards per page', () => {
        const form = quotesForm();
        radio('style', 'bold').click();
        radio('perPage', '8').click();
        expect(onChange).toHaveBeenCalledTimes(2);
        expect(form.options()).toEqual({quotes: [], style: 'bold', perPage: 8});
        radio('style', 'framed').click();
        radio('perPage', '4').click();
        expect(form.options()).toEqual({quotes: [], style: 'framed', perPage: 4});
    });

    it('follows a language change, keeping the options, until disposed', async () => {
        const form = quotesForm();
        await search('Einstein', EINSTEIN);
        tick(0);
        radio('style', 'bold').click();
        setLang('es');
        applyDom(box);
        expect(box.querySelector(`label[for="${query().id}"]`)?.textContent).toBe('Buscar citas de');
        expect(query().placeholder).toBe('Autor o personaje');
        expect(searchButton().textContent).toBe('Buscar');
        expect(person().textContent).toContain('Albert Einstein · 2 citas');
        expect(person().textContent).toContain('De Wikiquote');
        expect(q('#test-quotes-count').textContent).toBe('1 seleccionada');
        expect(q('#test-quotes-picked-title').textContent).toBe('Citas elegidas (1)');
        expect(pickedItems()[0].querySelector('button')?.getAttribute('aria-label'))
            .toBe('Quitar cita: Imagination is more important than knowledge.');
        expect(box.querySelector(`label[for="${own().id}"]`)?.textContent).toBe('Tus citas: Cita, Autor');
        expect(Array.from(q('#test-quotes-style').children, (r) => r.textContent)).toEqual(['Clásico', 'Negrita', 'Enmarcado']);
        expect(checked('style')).toBe('bold');
        expect((form.options().quotes as unknown[]).length).toBe(1);

        vi.mocked(SearchQuotes).mockReturnValueOnce(new Promise(() => {}) as never);
        searchButton().click();
        expect(status().textContent).toBe('Buscando en Wikiquote…');

        form.dispose?.();
        setLang('en');
        expect(person().textContent).toContain('2 citas');
        expect(Array.from(q('#test-quotes-style').children, (r) => r.textContent)).toEqual(['Clásico', 'Negrita', 'Enmarcado']);
    });

    it('says when a page in Spanish has no quotes', async () => {
        setLang('es');
        quotesForm();
        await search('X', result('Fulano', [], []));
        expect(person().textContent).toContain('No se encontraron citas en Fulano.');
        expect(q('#test-quotes-matches-field').hidden).toBe(true);
    });
});

describe('quoteLines', () => {
    it.each([
        ['Stay hungry, stay foolish, Steve Jobs', [{text: 'Stay hungry, stay foolish', author: 'Steve Jobs', source: ''}]],
        ['No comma at all', [{text: 'No comma at all', author: '', source: ''}]],
        ['  A , B  ', [{text: 'A', author: 'B', source: ''}]],
        ['\n\n  \n', []],
        ['One, A\r\nTwo, B\rThree', [
            {text: 'One', author: 'A', source: ''},
            {text: 'Two', author: 'B', source: ''},
            {text: 'Three', author: '', source: ''},
        ]],
        [', Nobody', [{text: '', author: 'Nobody', source: ''}]],
    ])('reads %j', (text, want) => {
        expect(quoteLines(text)).toEqual(want);
    });
});

// The Wikiquote search of the quote cards form (pageForms.ts): a query and
// Search, the person found with the other matches, the quotes as a
// checkbox list and the quotes picked so far, which outlive a new search
// so the user can collect quotes from several people. Also quoteLines, the
// typed quotes.

import {SearchQuotes} from '../wailsjs/go/main/App';
import {translated} from './fieldsets';
import {onLangChange, t, tn, type MessageKey} from './i18n';

// Quote is one quote as Go decodes it (Go: sheet.QuoteCard): source is
// "wikiquote" for a fetched quote, "" for a typed one.
export interface Quote {
    text: string;
    author: string;
    source: '' | 'wikiquote';
}

// SearchResult is what SearchQuotes resolves to (Go: quotes.Result).
interface SearchResult {
    person: string;
    matches: string[] | null;
    quotes: Array<{text: string}> | null;
}

// MAX_MATCHES caps the Other matches buttons (Go returns at most five).
const MAX_MATCHES = 5;

// quoteLines is the quotes typed in text, one per line: the text before
// the last comma and the author after it, both trimmed; a line without a
// comma is a quote without an author. Blank lines are dropped.
export function quoteLines(text: string): Quote[] {
    return text.split(/\r\n|\r|\n/).filter((line) => line.trim() !== '').map((line) => {
        const comma = line.lastIndexOf(',');
        if (comma < 0) return {text: line.trim(), author: '', source: ''};
        return {text: line.slice(0, comma).trim(), author: line.slice(comma + 1).trim(), source: ''};
    });
}

// sameQuote tells whether a and b are the same quote: same text and author.
function sameQuote(a: Quote, b: Quote): boolean {
    return a.text === b.text && a.author === b.author;
}

function errorText(err: unknown): string {
    if (err instanceof Error) return err.message;
    return String(err);
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, className = ''): HTMLElementTagNameMap[K] {
    const node = document.createElement(tag);
    if (className) node.className = className;
    return node;
}

// linkButton is a button that looks like a link, labelled by key.
function linkButton(id: string, key: MessageKey): HTMLButtonElement {
    const button = translated(el('button', 'link-button'), key);
    button.type = 'button';
    button.id = id;
    return button;
}

export interface QuoteSearch {
    // The picked quotes, in pick order.
    picked: () => Quote[];
    dispose: () => void;
}

// quoteSearch appends the search fields to container, with element IDs
// prefixed by idPrefix, starting with the picked quotes initial. onChange
// is called when the picked quotes change; a search alone changes nothing.
export function quoteSearch(container: HTMLElement, idPrefix: string, initial: Quote[], onChange: () => void): QuoteSearch {
    // Find quotes by: the query and Search, then the search status.
    const queryField = el('div', 'field quotes-query-field');
    const queryLabel = el('label');
    queryLabel.htmlFor = `${idPrefix}-query`;
    queryLabel.append(translated(el('span'), 'pages.quotesQuery'));
    const row = el('div', 'row');
    const query = el('input');
    query.type = 'search';
    query.id = `${idPrefix}-query`;
    query.dataset.i18nPlaceholder = 'pages.quotesQueryPlaceholder';
    query.placeholder = t('pages.quotesQueryPlaceholder');
    const searchButton = translated(el('button'), 'pages.quotesSearch');
    searchButton.type = 'button';
    searchButton.id = `${idPrefix}-search`;
    row.append(query, searchButton);
    const status = el('p', 'field-hint muted quotes-status');
    status.id = `${idPrefix}-status`;
    status.setAttribute('role', 'status');
    queryField.append(queryLabel, row, status);

    // The person found and the other matches.
    const person = el('div', 'quotes-person');
    person.id = `${idPrefix}-person`;
    person.hidden = true;
    const personText = el('p', 'quotes-person-name');
    const from = translated(el('p', 'field-hint muted'), 'pages.quotesFrom');
    person.append(personText, from);
    const matchesField = el('div', 'field quotes-matches-field');
    matchesField.id = `${idPrefix}-matches-field`;
    matchesField.hidden = true;
    const matchesLabel = translated(el('span'), 'pages.quotesMatches');
    matchesLabel.id = `${idPrefix}-matches-label`;
    const matches = el('div', 'quotes-matches');
    matches.id = `${idPrefix}-matches`;
    matches.setAttribute('role', 'group');
    matches.setAttribute('aria-labelledby', matchesLabel.id);
    matchesField.append(matchesLabel, matches);

    // The quotes found: All · None, the count and the checkbox list.
    const resultsField = el('div', 'field quotes-results-field');
    resultsField.id = `${idPrefix}-results-field`;
    resultsField.hidden = true;
    const bar = el('div', 'quotes-bar');
    const all = linkButton(`${idPrefix}-all`, 'pages.quotesAll');
    const none = linkButton(`${idPrefix}-none`, 'pages.quotesNoneButton');
    const count = el('span', 'quotes-count');
    count.id = `${idPrefix}-count`;
    bar.append(all, document.createTextNode(' · '), none, count);
    const results = el('div', 'quotes-list');
    results.id = `${idPrefix}-results`;
    results.tabIndex = 0;
    results.setAttribute('role', 'group');
    resultsField.append(bar, results);

    // The quotes picked so far, from any search, each with a remove button.
    const pickedField = el('div', 'field quotes-picked-field');
    pickedField.id = `${idPrefix}-picked-field`;
    pickedField.hidden = true;
    const pickedTitle = el('span');
    pickedTitle.id = `${idPrefix}-picked-title`;
    const pickedList = el('ul', 'quotes-picked');
    pickedList.id = `${idPrefix}-picked`;
    pickedList.setAttribute('aria-labelledby', pickedTitle.id);
    pickedField.append(pickedTitle, pickedList);

    container.append(queryField, person, matchesField, resultsField, pickedField);

    let picked: Quote[] = [];
    for (const quote of initial) {
        if (!picked.some((p) => sameQuote(p, quote))) picked.push(quote);
    }
    // The last result shown, its quotes as sent (author = the person).
    let shown: {person: string; matches: string[]; quotes: Quote[]} | undefined;
    // The search status: a message key while searching, Go's error text.
    let statusKey: MessageKey | undefined;
    // Each search's number; only the newest one's response is applied.
    let latest = 0;

    function setStatus(key?: MessageKey, error = '') {
        statusKey = key;
        status.textContent = key ? t(key) : error;
        status.classList.toggle('error', error !== '');
    }

    const isPicked = (quote: Quote) => picked.some((p) => sameQuote(p, quote));

    function setPicked(next: Quote[]) {
        const changed = next.length !== picked.length || next.some((q, i) => !sameQuote(q, picked[i]));
        picked = next;
        renderSelection();
        if (changed) onChange();
    }

    // renderSelection syncs the checkboxes, the count and the picked list
    // with picked, in the current language.
    function renderSelection() {
        const boxes = results.querySelectorAll<HTMLInputElement>('input[type="checkbox"]');
        let ticked = 0;
        shown?.quotes.forEach((quote, i) => {
            const on = isPicked(quote);
            boxes[i].checked = on;
            if (on) ticked++;
        });
        count.textContent = tn('pages.quotesSelected', ticked);
        pickedTitle.textContent = t('pages.quotesPicked', {n: picked.length});
        pickedField.hidden = picked.length === 0;
        pickedList.replaceChildren(...picked.map((quote) => {
            const item = el('li');
            const text = el('span', 'picked-text');
            text.textContent = quote.author ? `${quote.text} — ${quote.author}` : quote.text;
            const remove = el('button', 'icon-button');
            remove.type = 'button';
            remove.textContent = '×';
            remove.setAttribute('aria-label', t('pages.quotesRemove', {quote: quote.text}));
            remove.title = remove.getAttribute('aria-label') ?? '';
            remove.addEventListener('click', () => setPicked(picked.filter((p) => !sameQuote(p, quote))));
            item.append(text, remove);
            return item;
        }));
    }

    // renderResult shows the last result: the person, the other matches
    // and the quotes, in the current language.
    function renderResult() {
        if (!shown) return;
        const found = shown.quotes.length;
        personText.textContent = found > 0
            ? tn('pages.quotesFound', found, {person: shown.person})
            : t('pages.quotesNone', {person: shown.person});
        person.hidden = false;
        results.setAttribute('aria-label', t('pages.quotesResults'));
    }

    function showResult(result: SearchResult) {
        const quotes: Quote[] = [];
        for (const {text} of result.quotes ?? []) {
            const quote: Quote = {text, author: result.person, source: 'wikiquote'};
            if (!quotes.some((q) => sameQuote(q, quote))) quotes.push(quote);
        }
        shown = {person: result.person, matches: (result.matches ?? []).slice(0, MAX_MATCHES), quotes};

        matches.replaceChildren(...shown.matches.map((title) => {
            const button = el('button', 'quotes-match');
            button.type = 'button';
            button.textContent = title;
            button.addEventListener('click', () => {
                query.value = title;
                void search(title);
            });
            return button;
        }));
        matchesField.hidden = shown.matches.length === 0;

        results.replaceChildren(...quotes.map((quote) => {
            const label = el('label', 'check-row');
            const box = el('input');
            box.type = 'checkbox';
            box.addEventListener('change', () => {
                if (box.checked) setPicked(isPicked(quote) ? picked : [...picked, quote]);
                else setPicked(picked.filter((p) => !sameQuote(p, quote)));
            });
            const text = el('span');
            text.textContent = quote.text;
            label.append(box, text);
            return label;
        }));
        resultsField.hidden = quotes.length === 0;
        renderResult();
        renderSelection();
    }

    async function search(text: string) {
        const q = text.trim();
        if (q === '') return;
        const id = ++latest;
        searchButton.disabled = true;
        setStatus('pages.quotesSearching');
        try {
            const result = await SearchQuotes(q);
            if (id !== latest) return;
            setStatus();
            showResult(result);
        } catch (err) {
            if (id !== latest) return;
            setStatus(undefined, errorText(err));
        } finally {
            if (id === latest) searchButton.disabled = false;
        }
    }

    searchButton.addEventListener('click', () => void search(query.value));
    query.addEventListener('keydown', (e) => {
        if (e.key !== 'Enter') return;
        e.preventDefault();
        void search(query.value);
    });
    all.addEventListener('click', () => {
        if (shown) setPicked([...picked, ...shown.quotes.filter((q) => !isPicked(q))]);
    });
    none.addEventListener('click', () => {
        if (shown) setPicked(picked.filter((p) => !shown!.quotes.some((q) => sameQuote(p, q))));
    });

    renderSelection();
    const stopRelabel = onLangChange(() => {
        if (statusKey) status.textContent = t(statusKey);
        renderResult();
        renderSelection();
    });

    return {
        picked: () => picked.map((q) => ({...q})),
        dispose: stopRelabel,
    };
}

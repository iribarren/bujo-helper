// The "Print sheet" tab: preview the covers in a folder, select some, pick
// a format (a cover sheet, review cards, book spines, ticket stubs, a full
// page or a checklist), a paper, for a cover sheet or spines a cover size,
// for a cover sheet a cut mode and, for a full page or a checklist, a
// header, and export them to a PDF on that paper (see internal/sheet).

import {ChooseFolder, ExportPDF, ListCovers, OpenFile, PrintCutModes, PrintFormats, PrintPapers, PrintSizes} from '../wailsjs/go/main/App';
import {main, sheet} from '../wailsjs/go/models';

export interface PrintDeps {
    // The download output folder; the print sheet shows it until the user
    // picks another folder.
    outputFolder: () => string;
    showError: (message: string) => void;
    hideError: () => void;
    // The media types, empty until they load, and the Download tab's media
    // type ID; review cards follow it until the user picks another here.
    mediaTypes: () => main.MediaTypeInfo[];
    mediaType: () => string;
}

export interface PrintSheet {
    // Called whenever the tab is shown; reloads the folder.
    activate: () => void;
}

interface Card {
    item: HTMLLIElement;
    check: HTMLInputElement;
}

function byId<T extends HTMLElement>(id: string): T {
    const node = document.getElementById(id);
    if (!node) throw new Error(`missing #${id}`);
    return node as T;
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, className = '', text = ''): HTMLElementTagNameMap[K] {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text) node.textContent = text;
    return node;
}

function errorText(err: unknown): string {
    if (err instanceof Error) return err.message;
    return String(err);
}

function baseName(path: string): string {
    return path.split(/[\\/]/).pop() || path;
}

// REVIEW_CARDS is the ID of the review cards format, which asks what the
// cards are for to label their date field.
const REVIEW_CARDS = 'cards';

// HEADER_EXAMPLES are the header placeholders of the formats that print a
// header, by format ID; others use DEFAULT_HEADER_EXAMPLE.
const HEADER_EXAMPLES: Record<string, string> = {checklist: 'TBR 2026'};
const DEFAULT_HEADER_EXAMPLE = 'Books of 2026';

function plural(n: number, word: string): string {
    return `${n} ${word}${n === 1 ? '' : 's'}`;
}

// pageCount is the number of pages covers need at perPage covers a page.
export function pageCount(covers: number, perPage: number): number {
    return Math.ceil(covers / perPage);
}

// perPageOf is how many items of format fit on a page of paper, in the size
// sizeId when the format uses one, and below a header when withHeader and
// the format prints one, or undefined until they are known.
function perPageOf(
    paper: main.PrintPaperInfo | undefined,
    format: main.PrintFormatInfo | undefined,
    sizeId = '',
    withHeader = false,
): number | undefined {
    if (!paper || !format) return undefined;
    if (withHeader && format.usesHeader) return paper.headerPerPage?.[format.id];
    // Counts by size ID; a format without sizes has one, under ''.
    const counts: Record<string, number> | undefined = paper.perPage?.[format.id];
    return counts?.[format.usesSize ? sizeId : ''];
}

// sizeLabel names a cover size, with how many fit on a page once it is known.
function sizeLabel(size: main.PrintSizeInfo, perPage?: number): string {
    const label = `${size.label} · ${size.coverHeight} mm`;
    return perPage ? `${label} (${perPage} per page)` : label;
}

export function initPrint(deps: PrintDeps): PrintSheet {
    const ui = {
        dir: byId<HTMLInputElement>('print-dir'),
        browse: byId<HTMLButtonElement>('print-browse'),
        refresh: byId<HTMLButtonElement>('print-refresh'),
        selectAll: byId<HTMLButtonElement>('select-all'),
        selectNone: byId<HTMLButtonElement>('select-none'),
        selection: byId<HTMLSpanElement>('selection'),
        gallery: byId<HTMLUListElement>('gallery'),
        empty: byId<HTMLParagraphElement>('gallery-empty'),
        status: byId<HTMLSpanElement>('export-status'),
        openPdf: byId<HTMLButtonElement>('open-pdf'),
        exportPdf: byId<HTMLButtonElement>('export-pdf'),
        format: byId<HTMLSelectElement>('print-format'),
        media: byId<HTMLSelectElement>('print-media'),
        mediaField: byId<HTMLLabelElement>('print-media-field'),
        size: byId<HTMLSelectElement>('print-size'),
        sizeField: byId<HTMLLabelElement>('print-size-field'),
        paper: byId<HTMLSelectElement>('print-paper'),
        cut: byId<HTMLSelectElement>('print-cut'),
        cutField: byId<HTMLLabelElement>('print-cut-field'),
        header: byId<HTMLInputElement>('print-header'),
        headerField: byId<HTMLLabelElement>('print-header-field'),
    };

    // The folder picked with Browse…; empty means "follow the output folder".
    let chosenDir = '';
    let covers: main.CoverView[] = [];
    const selected = new Set<string>();
    let cards = new Map<string, Card>();
    // Increases with every load, so a slow, older listing cannot overwrite
    // a newer one.
    let loadSeq = 0;
    let exporting = false;
    let lastPdf = '';
    // The cover size presets, from PrintSizes; empty until they load.
    let sizes: main.PrintSizeInfo[] = [];
    // The paper size presets, from PrintPapers; empty until they load.
    let papers: main.PrintPaperInfo[] = [];
    // The print formats, from PrintFormats; empty until they load.
    let formats: main.PrintFormatInfo[] = [];
    // Whether the user picked what review cards are for in this tab; until
    // then it follows the Download tab's media type.
    let mediaChosen = false;

    function currentDir(): string {
        return chosenDir || deps.outputFolder();
    }

    function currentFormat(): main.PrintFormatInfo | undefined {
        return formats.find((f) => f.id === ui.format.value);
    }

    function currentMedia(): main.MediaTypeInfo | undefined {
        return deps.mediaTypes().find((t) => t.id === ui.media.value);
    }

    function currentSize(): main.PrintSizeInfo | undefined {
        return sizes.find((s) => s.id === ui.size.value);
    }

    function currentPaper(): main.PrintPaperInfo | undefined {
        return papers.find((p) => p.id === ui.paper.value);
    }

    // currentLayout is the chosen format, paper and, when the format uses
    // one, size, with how many items fit on a page, or undefined until they
    // have loaded.
    function currentLayout() {
        const format = currentFormat();
        const paper = currentPaper();
        if (!format || !paper) return undefined;
        const size = format.usesSize ? currentSize() : undefined;
        if (format.usesSize && !size) return undefined;
        // A header may leave room for fewer items (checklist rows).
        const withHeader = format.usesHeader && ui.header.value.trim() !== '';
        const perPage = perPageOf(paper, format, size?.id, withHeader);
        if (!perPage) return undefined;
        return {format, paper, size, perPage};
    }

    function updateSelection() {
        const n = selected.size;
        const layout = currentLayout();
        ui.selection.textContent = layout
            ? `${n} selected · ${plural(pageCount(n, layout.perPage), 'page')}`
            : `${n} selected`;
        ui.exportPdf.disabled = n === 0 || exporting || !layout;
        ui.selectAll.disabled = covers.length === 0 || n === covers.length;
        ui.selectNone.disabled = n === 0;
    }

    function setSelected(path: string, on: boolean) {
        if (on) selected.add(path);
        else selected.delete(path);
        const card = cards.get(path);
        if (card) {
            card.check.checked = on;
            card.item.classList.toggle('selected', on);
        }
    }

    function renderCard(cover: main.CoverView): Card {
        const item = el('li', 'card');
        const label = el('label', 'card-label');
        label.title = `${cover.name}\n${cover.width} × ${cover.height} px`;

        const check = el('input', 'card-check');
        check.type = 'checkbox';
        check.checked = selected.has(cover.path);
        item.classList.toggle('selected', check.checked);
        check.addEventListener('change', () => {
            setSelected(cover.path, check.checked);
            updateSelection();
        });
        // Space toggles a checkbox natively; Enter does too here.
        check.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                check.click();
            }
        });

        const frame = el('span', 'card-thumb');
        const img = el('img');
        img.loading = 'lazy';
        img.decoding = 'async';
        img.alt = '';
        img.src = cover.thumbUrl;
        frame.append(img);

        label.append(check, frame, el('span', 'card-name', cover.name));
        item.append(label);
        return {item, check};
    }

    function render() {
        cards = new Map();
        ui.gallery.replaceChildren(...covers.map((cover) => {
            const card = renderCard(cover);
            cards.set(cover.path, card);
            return card.item;
        }));
        ui.gallery.hidden = covers.length === 0;
        ui.empty.hidden = covers.length > 0;
        updateSelection();
    }

    function showEmpty(message: string) {
        covers = [];
        selected.clear();
        render();
        ui.empty.textContent = message;
    }

    async function load() {
        const dir = currentDir();
        ui.dir.value = dir;
        ui.dir.title = dir;
        const seq = ++loadSeq;
        ui.gallery.setAttribute('aria-busy', 'true');
        try {
            const list = await ListCovers(dir);
            if (seq !== loadSeq) return;
            covers = list ?? [];
            // Keep the selection of covers that are still there.
            const present = new Set(covers.map((c) => c.path));
            for (const path of [...selected]) {
                if (!present.has(path)) selected.delete(path);
            }
            render();
            ui.empty.textContent = 'No images in this folder';
        } catch (err) {
            if (seq !== loadSeq) return;
            showEmpty('No images to show');
            deps.showError(`Could not list covers: ${errorText(err)}`);
        } finally {
            if (seq === loadSeq) ui.gallery.removeAttribute('aria-busy');
        }
    }

    async function browse() {
        try {
            const dir = await ChooseFolder(currentDir());
            if (!dir) return; // dialog cancelled
            chosenDir = dir;
            deps.hideError();
            await load();
        } catch (err) {
            deps.showError(errorText(err));
        }
    }

    function selectAll(on: boolean) {
        for (const cover of covers) setSelected(cover.path, on);
        updateSelection();
    }

    async function exportPdf() {
        // Export in grid order, not in the order covers were clicked.
        const paths = covers.filter((c) => selected.has(c.path)).map((c) => c.path);
        const layout = currentLayout();
        if (paths.length === 0 || exporting || !layout) return;
        deps.hideError();
        const previous = {text: ui.status.textContent ?? '', title: ui.status.title, open: !ui.openPdf.hidden};
        exporting = true;
        updateSelection();
        ui.status.textContent = 'Exporting…';
        ui.status.title = '';
        ui.openPdf.hidden = true;
        try {
            const cards = layout.format.id === REVIEW_CARDS;
            const saved = await ExportPDF(paths, currentDir(), sheet.Options.createFrom({
                format: layout.format.id,
                size: layout.size?.id ?? '',
                paper: layout.paper.id,
                // The cut mode never changes the page count.
                cut: layout.format.usesCut ? ui.cut.value : '',
                dateLabel: cards ? currentMedia()?.dateLabel ?? '' : '',
                header: layout.format.usesHeader ? ui.header.value.trim() : '',
            }));
            if (saved === '') {
                // Dialog cancelled: leave the last result as it was.
                ui.status.textContent = previous.text;
                ui.status.title = previous.title;
                ui.openPdf.hidden = !previous.open;
                return;
            }
            lastPdf = saved;
            ui.status.textContent =
                `Saved ${baseName(saved)}: ${plural(paths.length, layout.format.noun || 'item')} on ${plural(pageCount(paths.length, layout.perPage), 'page')}`;
            ui.status.title = saved;
            ui.openPdf.hidden = false;
        } catch (err) {
            ui.status.textContent = '';
            ui.status.title = '';
            deps.showError(`Could not export PDF: ${errorText(err)}`);
        } finally {
            exporting = false;
            updateSelection();
        }
    }

    // updateSizeLabels shows how many covers or spines of each size fit on
    // the paper, in the chosen format.
    function updateSizeLabels() {
        const paper = currentPaper();
        const format = currentFormat();
        for (const option of Array.from(ui.size.options)) {
            const size = sizes.find((s) => s.id === option.value);
            if (!size) continue;
            const perPage = format?.usesSize ? perPageOf(paper, format, size.id) : undefined;
            option.textContent = sizeLabel(size, perPage);
        }
    }

    // updateFormat shows the controls that apply to the chosen format: the
    // cover size for a cover sheet or spines, the cut mode for a cover sheet,
    // what the cards are for for review cards, the header for a full page or
    // a checklist.
    function updateFormat() {
        const format = currentFormat();
        const usesSize = format?.usesSize ?? true;
        ui.size.disabled = !usesSize;
        ui.sizeField.classList.toggle('disabled', !usesSize);
        ui.cutField.hidden = !(format?.usesCut ?? true);
        ui.mediaField.hidden = ui.format.value !== REVIEW_CARDS;
        const usesHeader = format?.usesHeader ?? false;
        ui.header.disabled = !usesHeader;
        ui.headerField.hidden = !usesHeader;
        ui.header.placeholder = HEADER_EXAMPLES[ui.format.value] ?? DEFAULT_HEADER_EXAMPLE;
        updateSizeLabels();
        updateSelection();
    }

    // syncMedia fills the Cards for choices from the media types and, until
    // the user picks one here, selects the Download tab's media type.
    function syncMedia() {
        const types = deps.mediaTypes();
        if (ui.media.options.length !== types.length) {
            const kept = ui.media.value;
            ui.media.replaceChildren(...types.map((t) => {
                const option = el('option', '', t.label);
                option.value = t.id;
                option.title = `Date field: ${t.dateLabel}`;
                return option;
            }));
            if (mediaChosen) ui.media.value = kept;
        }
        if (!mediaChosen) ui.media.value = deps.mediaType();
    }

    async function loadPresets() {
        const [formatList, sizeList, paperList, cutList] =
            await Promise.allSettled([PrintFormats(), PrintSizes(), PrintPapers(), PrintCutModes()]);
        if (formatList.status === 'fulfilled') {
            formats = formatList.value ?? [];
            ui.format.replaceChildren(...formats.map((format) => {
                const option = el('option', '', format.label);
                option.value = format.id;
                option.selected = format.default;
                return option;
            }));
        } else {
            deps.showError(`Could not load print formats: ${errorText(formatList.reason)}`);
        }
        if (sizeList.status === 'fulfilled') {
            sizes = sizeList.value ?? [];
            ui.size.replaceChildren(...sizes.map((size) => {
                const option = el('option', '', sizeLabel(size));
                option.value = size.id;
                option.selected = size.default;
                return option;
            }));
        } else {
            deps.showError(`Could not load cover sizes: ${errorText(sizeList.reason)}`);
        }
        if (paperList.status === 'fulfilled') {
            papers = paperList.value ?? [];
            ui.paper.replaceChildren(...papers.map((paper) => {
                const option = el('option', '', paper.label);
                option.value = paper.id;
                option.title = `${paper.width} × ${paper.height} mm`;
                option.selected = paper.default;
                return option;
            }));
        } else {
            deps.showError(`Could not load paper sizes: ${errorText(paperList.reason)}`);
        }
        if (cutList.status === 'fulfilled') {
            ui.cut.replaceChildren(...(cutList.value ?? []).map((cut) => {
                const option = el('option', '', cut.label);
                option.value = cut.id;
                option.selected = cut.default;
                return option;
            }));
        } else {
            deps.showError(`Could not load cut modes: ${errorText(cutList.reason)}`);
        }
        updateFormat();
    }

    async function openPdf() {
        try {
            await OpenFile(lastPdf);
        } catch (err) {
            deps.showError(errorText(err));
        }
    }

    ui.browse.addEventListener('click', browse);
    ui.refresh.addEventListener('click', () => {
        deps.hideError();
        load();
    });
    ui.selectAll.addEventListener('click', () => selectAll(true));
    ui.selectNone.addEventListener('click', () => selectAll(false));
    ui.exportPdf.addEventListener('click', exportPdf);
    ui.openPdf.addEventListener('click', openPdf);
    ui.format.addEventListener('change', () => {
        syncMedia();
        updateFormat();
    });
    ui.media.addEventListener('change', () => {
        mediaChosen = true;
    });
    ui.size.addEventListener('change', updateSelection);
    // A header may change the page count.
    ui.header.addEventListener('input', updateSelection);
    ui.paper.addEventListener('change', () => {
        updateSizeLabels();
        updateSelection();
    });
    updateSelection();
    loadPresets();

    return {
        activate: () => {
            syncMedia();
            load();
        },
    };
}

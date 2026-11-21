// The "Print covers" tab: preview the covers in a folder, select some, pick
// a format (a cover sheet, review cards, book spines, ticket stubs, a full
// page, a checklist or polaroids), a paper, for a cover sheet, spines or
// polaroids a cover size, for a cover sheet a crop mode (a standard print
// shape the photos are cropped to) and a cut mode and, for a full page or a
// checklist, a header and, for those and review cards, the lettering font
// and Colour-in, and whether to end with a credits page for the
// images that have a credit, and export them to a PDF on that paper (see
// internal/sheet). The Gallery | Preview switch shows, instead of the
// gallery, a live preview of the PDF of the selected covers (preview.ts).

import {ChooseFolder, ExportPDF, ListCovers, OpenFile, PreviewPDF, PrintCropModes, PrintCutModes, PrintFormats, PrintPages, PrintPapers, PrintSizes} from '../wailsjs/go/main/App';
import {main, sheet} from '../wailsjs/go/models';
import {fillSelect, labelOptions, renderSharedFieldsets, syncFieldsets} from './fieldsets';
import {getLang, has, label, onLangChange, pluralKey, t, tn, type MessageKey} from './i18n';
import {letteringFonts} from './lettering';
import {initPreview} from './preview';
import {initRadioGroup} from './radioGroup';

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
    // The preview's debounce delay in milliseconds; preview.ts's default
    // when unset.
    previewDelay?: number;
}

export interface PrintSheet {
    // Called whenever the tab is shown; reloads the folder.
    activate: () => void;
    // The folder shown: the one picked with Browse…, or the output folder.
    folder: () => string;
    // Shows dir from now on, as if picked with Browse…; the next activate
    // loads it.
    showFolder: (dir: string) => void;
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

// headerExample is the header placeholder of a format that prints a header:
// the message "print.headerExample.<format ID>", or "print.headerExample".
function headerExample(formatId: string): string {
    const key = `print.headerExample.${formatId}`;
    return has(key) ? t(key) : t('print.headerExample');
}

// dateLabelOf is the translated date field label of review cards for a
// media type, e.g. "Read on".
function dateLabelOf(media: main.MediaTypeInfo): string {
    return label('date', media.id, media.dateLabel);
}

// pageCount is the number of pages covers need at perPage covers a page:
// the estimate shown until PrintPages counts them from the images.
export function pageCount(covers: number, perPage: number): number {
    return Math.ceil(covers / perPage);
}

// squarePerPageOf is how many square images fit on a page of paper in the
// size sizeId, for a format that gives them square cells (a cover sheet),
// or undefined for other formats.
function squarePerPageOf(
    paper: main.PrintPaperInfo | undefined,
    format: main.PrintFormatInfo | undefined,
    sizeId: string,
): number | undefined {
    if (!paper || !format) return undefined;
    return paper.squarePerPage?.[format.id]?.[sizeId];
}

// SelectionShape is what PrintPages found about the shapes of the selected
// images: whether every one is square, and how many images like them fit
// on a page at each size where every one gets a wide cell (see
// main.PrintPagesInfo.widePerPage).
export interface SelectionShape {
    allSquare: boolean;
    widePerPage?: Record<string, number> | null;
}

// activeCrop is the crop mode that applies to format: crop, unless it is
// None (the default) or the format does not crop; undefined until they load.
export function activeCrop(
    format: main.PrintFormatInfo | undefined,
    crop: main.PrintCropModeInfo | undefined,
): main.PrintCropModeInfo | undefined {
    return format?.usesCrop && crop && !crop.default ? crop : undefined;
}

// usesSizeOf is whether the cover size applies to format with crop: when
// the format uses one, unless the crop mode has a fixed size. True until
// the formats load.
export function usesSizeOf(format: main.PrintFormatInfo | undefined, crop: main.PrintCropModeInfo | undefined): boolean {
    return (format?.usesSize ?? true) && (activeCrop(format, crop)?.usesSize ?? true);
}

// usesCutOf is whether the cut mode applies to format with crop: when the
// format uses one, unless the crop mode draws a frame instead. True until
// the formats load.
export function usesCutOf(format: main.PrintFormatInfo | undefined, crop: main.PrintCropModeInfo | undefined): boolean {
    return (format?.usesCut ?? true) && (activeCrop(format, crop)?.usesCut ?? true);
}

// sizePerPageOf is how many of the selected images fit on a page of paper
// in the size sizeId, for the size labels: with a crop mode, how many
// photos of that shape; on a cover sheet, the square count when every
// image is square, the wide count when every image gets a wide cell at
// that size (landscape images, 4:5 portraits), and the 2:3 count otherwise
// or until PrintPages answers. It is undefined for a format or crop mode
// without sizes, or until the counts load.
export function sizePerPageOf(
    paper: main.PrintPaperInfo | undefined,
    format: main.PrintFormatInfo | undefined,
    sizeId: string,
    shape?: SelectionShape,
    crop?: main.PrintCropModeInfo,
): number | undefined {
    if (!paper || !format?.usesSize || !usesSizeOf(format, crop)) return undefined;
    if (activeCrop(format, crop)) return perPageOf(paper, format, sizeId, false, crop);
    const shaped = shape?.allSquare ? squarePerPageOf(paper, format, sizeId) : shape?.widePerPage?.[sizeId];
    return shaped ?? perPageOf(paper, format, sizeId);
}

// perPageOf is how many items of format fit on a page of paper, in the size
// sizeId when the format uses one, below a header when withHeader and the
// format prints one, and cropped to crop when the format crops (photos, for
// a photo strip), or undefined until they are known.
export function perPageOf(
    paper: main.PrintPaperInfo | undefined,
    format: main.PrintFormatInfo | undefined,
    sizeId = '',
    withHeader = false,
    crop?: main.PrintCropModeInfo,
): number | undefined {
    if (!paper || !format) return undefined;
    const cropped = activeCrop(format, crop);
    if (cropped) {
        // Counts by size ID; a crop mode without sizes has one, under ''.
        const counts: Record<string, number> | undefined = paper.cropPerPage?.[cropped.id];
        return counts?.[cropped.usesSize ? sizeId : ''];
    }
    if (withHeader && format.usesHeader) return paper.headerPerPage?.[format.id];
    // Counts by size ID; a format without sizes has one, under ''.
    const counts: Record<string, number> | undefined = paper.perPage?.[format.id];
    return counts?.[format.usesSize ? sizeId : ''];
}

// sizeLabel names a cover size, with how many fit on a page once it is known.
function sizeLabel(size: main.PrintSizeInfo, perPage?: number): string {
    const name = `${label('size', size.id, size.label)} · ${size.coverHeight} mm`;
    return perPage ? t('print.perPage', {label: name, n: perPage}) : name;
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
        crop: byId<HTMLSelectElement>('print-crop'),
        cropField: byId<HTMLLabelElement>('print-crop-field'),
        options: byId<HTMLElement>('print-options'),
        view: byId<HTMLDivElement>('print-view'),
        pager: byId<HTMLDivElement>('print-pager'),
        preview: byId<HTMLDivElement>('print-preview'),
        sheet: byId<HTMLDivElement>('print-sheet'),
    };
    // Gallery | Preview; the preview renders only while it is shown.
    const viewGroup = initRadioGroup(ui.view, () => showView());
    const preview = initPreview({
        sheet: ui.sheet,
        prev: byId<HTMLButtonElement>('print-prev'),
        next: byId<HTMLButtonElement>('print-next'),
        pageLabel: byId<HTMLSpanElement>('print-preview-count'),
        message: byId<HTMLParagraphElement>('print-preview-message'),
    }, {delay: deps.previewDelay});
    // Page (paper, cut mode, credits page) and Lettering (the header, font
    // and Colour-in), as in Make pages. The tab keeps its own font and
    // Colour-in while the user switches tabs.
    const shared = renderSharedFieldsets({
        prefix: 'print',
        container: ui.options,
        paperTitle: 'print.paperTitle',
        cutTitle: 'print.cutTitle',
        credits: {label: 'print.credits', title: 'print.creditsTitle'},
        text: {id: 'header', label: 'print.header', title: 'print.headerTitle'},
        hint: 'print.noHeader',
        onPaperChange: () => {
            updateSizeLabels();
            updateSelection();
        },
        // The font may change the credits page's length; both are printed.
        onLetteringChange: () => updateSelection(),
    });
    const paperGroup = shared.paper;
    const cutSelect = shared.cut;
    const headerInput = shared.text;
    const creditsBox = shared.credits!;

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
    // The crop modes, from PrintCropModes; empty until they load.
    let crops: main.PrintCropModeInfo[] = [];
    // Whether the user picked what review cards are for in this tab; until
    // then it follows the Download tab's media type.
    let mediaChosen = false;
    // The page count of the selection from PrintPages, for the request it
    // answers (key: paths and settings; pathsKey: paths only; shapeKey:
    // paths, format and paper, which set its wide counts); undefined until
    // it arrives. pagesKey is the latest request; pagesSeq increases with
    // every request, so a slow, older answer cannot overwrite a newer one.
    let exact: {key: string; pathsKey: string; shapeKey: string; info: main.PrintPagesInfo} | undefined;
    let pagesKey = '';
    let pagesSeq = 0;
    // The message of the empty gallery, and the last saved PDF's status
    // (shown while lastExport is set), to show them again in another language.
    let emptyKey: MessageKey = 'print.emptyFolder';
    let lastExport: {file: string; formatId: string; items: number; pages: number} | undefined;
    // Increases with every listing, so the preview renders again when the
    // images may have changed (Refresh).
    let listing = 0;

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
        return papers.find((p) => p.id === paperGroup.value);
    }

    function currentCrop(): main.PrintCropModeInfo | undefined {
        return crops.find((c) => c.id === ui.crop.value);
    }

    // currentLayout is the chosen format, paper and, when they apply, crop
    // mode and size, with how many items fit on a page, or undefined until
    // they have loaded.
    function currentLayout() {
        const format = currentFormat();
        const paper = currentPaper();
        if (!format || !paper) return undefined;
        const crop = activeCrop(format, currentCrop());
        const usesSize = usesSizeOf(format, crop);
        const size = usesSize ? currentSize() : undefined;
        if (usesSize && !size) return undefined;
        // A header may leave room for fewer items (checklist rows).
        const withHeader = format.usesHeader && headerInput.value.trim() !== '';
        const perPage = perPageOf(paper, format, size?.id, withHeader, crop);
        if (!perPage) return undefined;
        return {format, paper, size, crop, perPage};
    }

    // selectedPaths are the selected covers, in grid order.
    function selectedPaths(): string[] {
        return covers.filter((c) => selected.has(c.path)).map((c) => c.path);
    }

    type Layout = NonNullable<ReturnType<typeof currentLayout>>;

    // printOptions are the print settings of layout.
    function printOptions(layout: Layout): sheet.Options {
        const media = currentMedia();
        return sheet.Options.createFrom({
            format: layout.format.id,
            size: layout.size?.id ?? '',
            paper: layout.paper.id,
            crop: layout.crop?.id ?? '',
            // The cut mode never changes the page count.
            cut: usesCutOf(layout.format, layout.crop) ? cutSelect.value : '',
            // Translated here: Go prints the label as given.
            dateLabel: layout.format.id === REVIEW_CARDS && media ? dateLabelOf(media) : '',
            header: layout.format.usesHeader ? headerInput.value.trim() : '',
            // The default output for formats without lettering.
            font: layout.format.usesLettering ? shared.lettering.options().font : '',
            colourIn: layout.format.usesLettering ? shared.lettering.options().colourIn : false,
            credits: creditsBox.checked,
        });
    }

    // requestKey identifies a page count request: what changes the count.
    function requestKey(paths: string[], opts: sheet.Options): string {
        // The font joins a Fonts section to a credits page.
        return JSON.stringify([paths, opts.format, opts.size, opts.paper, opts.crop, opts.header, opts.font, opts.credits]);
    }

    // pagesOf is the page count of paths with layout: exact once PrintPages
    // has answered for them, the estimate until then.
    function pagesOf(paths: string[], layout: Layout): number {
        const key = requestKey(paths, printOptions(layout));
        return exact?.key === key && exact.info.pages > 0 ? exact.info.pages : pageCount(paths.length, layout.perPage);
    }

    // shapeKey identifies what the wide counts of PrintPages depend on.
    function shapeKey(paths: string[], format: string, paper: string, crop: string): string {
        return JSON.stringify([paths, format, paper, crop]);
    }

    // selectionShape is what PrintPages found about the shapes of the
    // selected images, or undefined until it answers for them; its wide
    // counts only for the chosen format and paper.
    function selectionShape(): SelectionShape | undefined {
        const paths = selectedPaths();
        if (exact?.pathsKey !== JSON.stringify(paths)) return undefined;
        const crop = activeCrop(currentFormat(), currentCrop())?.id ?? '';
        const sameShape = exact.shapeKey === shapeKey(paths, ui.format.value, paperGroup.value, crop);
        return {allSquare: exact.info.allSquare, widePerPage: sameShape ? exact.info.widePerPage : undefined};
    }

    // countPages asks PrintPages for the exact page count of paths with
    // layout, unless that is the latest request already; the answer
    // updates the toolbar and the size labels. On failure the estimate
    // stays until the next request.
    async function countPages(paths: string[], layout: Layout) {
        const opts = printOptions(layout);
        const key = requestKey(paths, opts);
        if (key === pagesKey) return;
        pagesKey = key;
        const seq = ++pagesSeq;
        try {
            const info = await PrintPages(paths, opts);
            if (seq !== pagesSeq) return;
            exact = {key, pathsKey: JSON.stringify(paths), shapeKey: shapeKey(paths, opts.format, opts.paper, opts.crop), info};
            updateSizeLabels();
            updateSelection();
        } catch (err) {
            if (seq !== pagesSeq) return;
            // Forget the key so the same request can be retried.
            pagesKey = '';
            console.warn(`Could not count pages: ${errorText(err)}`);
        }
    }

    function updateSelection() {
        const n = selected.size;
        const layout = currentLayout();
        const paths = selectedPaths();
        ui.selection.textContent = layout
            ? t('print.selection', {selected: tn('print.selected', n), pages: tn('print.pages', pagesOf(paths, layout))})
            : tn('print.selected', n);
        if (layout) countPages(paths, layout);
        ui.exportPdf.disabled = n === 0 || exporting || !layout;
        ui.selectAll.disabled = covers.length === 0 || n === covers.length;
        ui.selectNone.disabled = n === 0;
        updatePreview();
    }

    function previewing(): boolean {
        return viewGroup.value === 'preview';
    }

    // showView shows the gallery (or its empty message) or the preview.
    function showView() {
        const on = previewing();
        ui.gallery.hidden = on || covers.length === 0;
        ui.empty.hidden = on || covers.length > 0;
        ui.preview.hidden = !on;
        ui.pager.hidden = !on;
        if (on) updatePreview();
        else preview.cancel();
    }

    // updatePreview previews the selected covers with the current options,
    // while the preview is shown: the PDF Export PDF would save.
    function updatePreview() {
        if (!previewing()) return;
        const paths = selectedPaths();
        if (paths.length === 0) {
            preview.showMessage('preview.noSelection');
            return;
        }
        const layout = currentLayout();
        if (!layout) return;
        // The page keeps the paper's shape while the PDF renders.
        ui.sheet.style.setProperty('--page-ratio', String(layout.paper.width / layout.paper.height));
        const opts = printOptions(layout);
        // Go prints stub, card and credits labels in the UI language.
        const key = JSON.stringify([paths, opts, listing, getLang()]);
        preview.schedule({key, render: () => PreviewPDF(paths, opts)});
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
        // The images may have changed: count the pages and preview again.
        pagesKey = '';
        listing++;
        cards = new Map();
        ui.gallery.replaceChildren(...covers.map((cover) => {
            const card = renderCard(cover);
            cards.set(cover.path, card);
            return card.item;
        }));
        ui.gallery.hidden = previewing() || covers.length === 0;
        ui.empty.hidden = previewing() || covers.length > 0;
        updateSelection();
    }

    function showEmpty(key: MessageKey) {
        covers = [];
        selected.clear();
        render();
        emptyKey = key;
        ui.empty.textContent = t(key);
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
            emptyKey = 'print.emptyFolder';
            ui.empty.textContent = t(emptyKey);
        } catch (err) {
            if (seq !== loadSeq) return;
            showEmpty('print.emptyError');
            deps.showError(t('print.errList', {error: errorText(err)}));
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
        const paths = selectedPaths();
        const layout = currentLayout();
        if (paths.length === 0 || exporting || !layout) return;
        deps.hideError();
        const previous = {
            text: ui.status.textContent ?? '',
            title: ui.status.title,
            open: !ui.openPdf.hidden,
            export: lastExport,
        };
        exporting = true;
        updateSelection();
        lastExport = undefined;
        ui.status.textContent = t('print.exporting');
        ui.status.title = '';
        ui.openPdf.hidden = true;
        try {
            const saved = await ExportPDF(paths, currentDir(), printOptions(layout));
            if (saved === '') {
                // Dialog cancelled: leave the last result as it was.
                lastExport = previous.export;
                ui.status.textContent = previous.text;
                ui.status.title = previous.title;
                ui.openPdf.hidden = !previous.open;
                renderExportStatus();
                return;
            }
            lastPdf = saved;
            lastExport = {
                file: baseName(saved),
                formatId: layout.format.id,
                items: paths.length,
                pages: pagesOf(paths, layout),
            };
            renderExportStatus();
            ui.status.title = saved;
            ui.openPdf.hidden = false;
        } catch (err) {
            ui.status.textContent = '';
            ui.status.title = '';
            deps.showError(t('print.errExport', {error: errorText(err)}));
        } finally {
            exporting = false;
            updateSelection();
        }
    }

    // renderExportStatus shows the last saved PDF, e.g. "Saved x.pdf: 9
    // covers on 1 page", in the current language.
    function renderExportStatus() {
        if (!lastExport) return;
        const {file, formatId, items, pages} = lastExport;
        ui.status.textContent = t('print.saved', {
            file,
            items: tn(pluralKey('noun', formatId, 'noun.item'), items),
            pages: tn('print.pages', pages),
        });
    }

    // updateSizeLabels shows how many covers, spines, polaroids or cropped
    // photos of each size fit on the paper, in the chosen format: on a
    // cover sheet, how many square covers when every selected image is
    // square, and how many like the selection when every one gets a wide
    // cell (see sizePerPageOf).
    function updateSizeLabels() {
        const paper = currentPaper();
        const format = currentFormat();
        const shape = selectionShape();
        const crop = currentCrop();
        for (const option of Array.from(ui.size.options)) {
            const size = sizes.find((s) => s.id === option.value);
            if (!size) continue;
            option.textContent = sizeLabel(size, sizePerPageOf(paper, format, size.id, shape, crop));
        }
    }

    // updateFormat shows the controls that apply to the chosen format and
    // crop mode: the cover size for a cover sheet, spines or polaroids
    // (hidden for a crop mode with a fixed size), the crop mode for a cover
    // sheet, the cut mode for a cover sheet unless the crop mode draws a
    // frame, what the cards are for for review cards, the header for a full
    // page or a checklist (a hint otherwise), the font and Colour-in for
    // those and review cards; and the option groups that have a field shown.
    function updateFormat() {
        const format = currentFormat();
        const crop = currentCrop();
        const formatUsesSize = format?.usesSize ?? true;
        ui.size.disabled = !formatUsesSize;
        ui.sizeField.classList.toggle('disabled', !formatUsesSize);
        ui.sizeField.hidden = formatUsesSize && !usesSizeOf(format, crop);
        ui.cropField.hidden = !(format?.usesCrop ?? true);
        shared.cutField.hidden = !usesCutOf(format, crop);
        ui.mediaField.hidden = ui.format.value !== REVIEW_CARDS;
        // Lettering shows the header, or a hint that the format prints none,
        // and the font and Colour-in for the formats that print lettering.
        shared.showText(format?.usesHeader ?? false);
        shared.showLettering(format?.usesLettering ?? false);
        headerInput.placeholder = headerExample(ui.format.value);
        syncFieldsets(ui.options);
        updateSizeLabels();
        updateSelection();
    }

    // syncMedia fills the Cards for choices from the media types and, until
    // the user picks one here, selects the Download tab's media type.
    function syncMedia() {
        const types = deps.mediaTypes();
        if (ui.media.options.length !== types.length) {
            const kept = ui.media.value;
            ui.media.replaceChildren(...types.map((type) => {
                const option = el('option');
                option.value = type.id;
                return option;
            }));
            if (mediaChosen) ui.media.value = kept;
        }
        labelMedia();
        if (!mediaChosen) ui.media.value = deps.mediaType();
    }

    // labelMedia names the Cards for choices, with their date field.
    function labelMedia() {
        for (const option of Array.from(ui.media.options)) {
            const type = deps.mediaTypes().find((m) => m.id === option.value);
            if (!type) continue;
            option.textContent = label('media', type.id, type.label);
            option.title = t('print.dateField', {label: dateLabelOf(type)});
        }
    }

    // labelViews names the Gallery | Preview switch in the current language.
    function labelViews() {
        ui.view.setAttribute('aria-label', t('print.view'));
        viewGroup.replaceOptions([
            {value: 'gallery', label: t('print.viewGallery')},
            {value: 'preview', label: t('print.viewPreview')},
        ]);
    }

    // relabel shows the tab's dynamic text in the current language, keeping
    // the selections and the header.
    function relabel() {
        labelViews();
        preview.relabel();
        labelOptions(ui.format, 'format', formats);
        shared.relabel();
        labelOptions(ui.crop, 'crop', crops);
        labelMedia();
        updateSizeLabels();
        headerInput.placeholder = headerExample(ui.format.value);
        if (ui.empty.textContent) ui.empty.textContent = t(emptyKey);
        if (exporting) ui.status.textContent = t('print.exporting');
        else renderExportStatus();
        updateSelection();
    }

    async function loadPresets() {
        const [formatList, sizeList, paperList, cutList, cropList, fontList] = await Promise.allSettled(
            [PrintFormats(), PrintSizes(), PrintPapers(), PrintCutModes(), PrintCropModes(), letteringFonts()]);
        if (formatList.status === 'fulfilled') {
            formats = formatList.value ?? [];
            ui.format.replaceChildren(...formats.map((format) => {
                const option = el('option', '', label('format', format.id, format.label));
                option.value = format.id;
                option.selected = format.default;
                return option;
            }));
        } else {
            deps.showError(t('print.errFormats', {error: errorText(formatList.reason)}));
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
            deps.showError(t('print.errSizes', {error: errorText(sizeList.reason)}));
        }
        if (paperList.status === 'fulfilled') {
            papers = paperList.value ?? [];
            shared.setPapers(papers);
        } else {
            deps.showError(t('print.errPapers', {error: errorText(paperList.reason)}));
        }
        if (cutList.status === 'fulfilled') {
            shared.setCuts(cutList.value ?? []);
        } else {
            deps.showError(t('print.errCuts', {error: errorText(cutList.reason)}));
        }
        if (cropList.status === 'fulfilled') {
            crops = cropList.value ?? [];
            fillSelect(ui.crop, 'crop', crops);
        } else {
            deps.showError(t('print.errCrops', {error: errorText(cropList.reason)}));
        }
        if (fontList.status === 'fulfilled') {
            shared.lettering.setFonts(fontList.value);
        } else {
            deps.showError(t('lettering.errFonts', {error: errorText(fontList.reason)}));
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
        // The cards' date label is printed.
        updatePreview();
    });
    // The cut mode never changes the count, only the preview.
    cutSelect.addEventListener('change', updatePreview);
    ui.size.addEventListener('change', updateSelection);
    // A crop mode shows or hides the size and cut mode, and sets the count.
    ui.crop.addEventListener('change', updateFormat);
    // A header may change the page count.
    headerInput.addEventListener('input', updateSelection);
    // Credits pages follow the images when a selected image has a credit.
    creditsBox.addEventListener('change', updateSelection);
    onLangChange(relabel);
    labelViews();
    updateSelection();
    loadPresets();

    return {
        activate: () => {
            syncMedia();
            load();
        },
        folder: currentDir,
        showFolder: (dir) => {
            chosenDir = dir;
        },
    };
}

// The "Make pages" tab: generated pages that need no images. Pick a page
// kind (from PageKinds), set its own options (its form, see pageForms.ts)
// and the shared Page and Lettering options (paper, cut mode, title, font
// and Colour-in), see how many pages it makes,
// and export it to a PDF (see internal/sheet.BuildPage). A live preview
// (preview.ts) draws the PDF PreviewPage renders for the current options,
// the same one Export PDF saves.

import {ExportPage, OpenFile, PageKinds, PreviewPage, PrintCutModes, PrintPapers, SavePagesPrefs} from '../wailsjs/go/main/App';
import {main, settings, sheet} from '../wailsjs/go/models';
import {renderSharedFieldsets, syncFieldsets} from './fieldsets';
import {label, onLangChange, t, tn} from './i18n';
import {letteringFonts} from './lettering';
import {pageForms, type PageForm} from './pageForms';
import {prefsSaver} from './prefs';
import {initPreview} from './preview';
import {initRadioGroup} from './radioGroup';
import {chooseShown, visibleItems, type View} from './view';

export interface PagesDeps {
    // The folder the save dialog starts in.
    defaultDir: () => string;
    showError: (message: string) => void;
    hideError: () => void;
    // The preview's debounce delay in milliseconds; preview.ts's default
    // when unset.
    previewDelay?: number;
    // The kind and shared options remembered from the last run
    // (GetSettings); the defaults when unset.
    savedPrefs?: settings.PagesPrefs;
    // The list items shown at first (see applyView); every one when unset.
    view?: View;
}

export interface PagesTab {
    // Shows only the page kinds, papers and cut modes the view shows,
    // keeping the chosen ones that are still shown, else the remembered
    // ones, else the shown default. Nothing is saved; the same view again
    // changes nothing. Each kind keeps its form options while hidden.
    applyView: (view: View) => void;
}

function byId<T extends HTMLElement>(id: string): T {
    const node = document.getElementById(id);
    if (!node) throw new Error(`missing #${id}`);
    return node as T;
}

function errorText(err: unknown): string {
    if (err instanceof Error) return err.message;
    return String(err);
}

function baseName(path: string): string {
    return path.split(/[\\/]/).pop() || path;
}

// parseDefaults reads a kind's default options, a JSON object; malformed
// JSON reads as no options.
export function parseDefaults(json: string): Record<string, unknown> {
    try {
        const value: unknown = JSON.parse(json || '{}');
        return value && typeof value === 'object' ? value as Record<string, unknown> : {};
    } catch {
        return {};
    }
}

export function initPages(deps: PagesDeps): PagesTab {
    const ui = {
        kindTitle: byId<HTMLHeadingElement>('pages-kind-title'),
        forms: byId<HTMLDivElement>('pages-form'),
        options: byId<HTMLElement>('pages-options'),
        count: byId<HTMLSpanElement>('pages-count'),
        sheet: byId<HTMLDivElement>('pages-sheet'),
        prev: byId<HTMLButtonElement>('pages-prev'),
        next: byId<HTMLButtonElement>('pages-next'),
        message: byId<HTMLParagraphElement>('pages-preview-message'),
        status: byId<HTMLSpanElement>('pages-status'),
        openPdf: byId<HTMLButtonElement>('pages-open-pdf'),
        exportPdf: byId<HTMLButtonElement>('pages-export'),
    };
    const preview = initPreview(
        {sheet: ui.sheet, prev: ui.prev, next: ui.next, pageLabel: ui.count, message: ui.message},
        {delay: deps.previewDelay},
    );
    // The remembered options; saving starts once they are restored. The
    // kinds' own form options are not remembered.
    const saved = deps.savedPrefs ?? new settings.PagesPrefs();
    const saver = prefsSaver('Make pages', currentPrefs, SavePagesPrefs);
    // The list items shown, and the view as applied last.
    let view = deps.view;
    let viewKey = JSON.stringify(view ?? null);
    // changed previews a user change and remembers it.
    const changed = () => {
        update();
        saver.save();
    };
    // The page kinds, a radio group; the user's choice calls update.
    const kindGroup = initRadioGroup(byId<HTMLDivElement>('pages-kinds'), changed);
    // Page (paper, cut mode) and Lettering (the title, font and Colour-in),
    // as in Print covers, without a credits page: generated pages have no
    // images to credit. Each tab keeps its own font and Colour-in, like its
    // paper, while the user switches tabs.
    const shared = renderSharedFieldsets({
        prefix: 'pages',
        container: ui.options,
        paperTitle: 'pages.paperTitle',
        cutTitle: 'pages.cutTitle',
        text: {id: 'title', label: 'pages.title', title: 'pages.titleTitle'},
        hint: 'pages.noTitle',
        view,
        onPaperChange: changed,
        onLetteringChange: changed,
    });

    // The page kinds, papers and cut modes from Go, every one the view may
    // show; empty until they load.
    let kinds: main.PageKindInfo[] = [];
    let papers: main.PrintPaperInfo[] = [];
    // The forms built so far, by kind ID; each keeps its options while
    // another kind is shown.
    const forms = new Map<string, {box: HTMLElement; form?: PageForm}>();
    let exporting = false;
    let lastPdf = '';
    // The last saved PDF's status, to show it again in another language.
    let lastExport: {file: string; pages?: number} | undefined;

    // currentPrefs are the kind and shared options to remember: ids, never
    // labels. A list that failed to load keeps the remembered choice.
    function currentPrefs(): settings.PagesPrefs {
        const {font, colourIn} = shared.lettering.options();
        return settings.PagesPrefs.createFrom({
            kind: kindGroup.value || saved.kind || '',
            paper: shared.paper.value || saved.paper || '',
            cut: shared.cut.value || saved.cut || '',
            title: shared.text.value.trim(),
            font: font || saved.font || '',
            colourIn,
        });
    }

    function currentKind(): main.PageKindInfo | undefined {
        return kinds.find((k) => k.id === kindGroup.value);
    }

    function currentPaper(): main.PrintPaperInfo | undefined {
        return papers.find((p) => p.id === shared.paper.value);
    }

    // formOf is the form of kind, built on first use; undefined for a kind
    // without one.
    function formOf(kind: main.PageKindInfo): {box: HTMLElement; form?: PageForm} {
        let entry = forms.get(kind.id);
        if (!entry) {
            const box = document.createElement('div');
            box.id = `pages-form-${kind.id}`;
            box.className = 'kind-form';
            ui.forms.append(box);
            const build = pageForms[kind.id];
            const form = build?.(box, `pages-${kind.id}`, parseDefaults(kind.defaults), () => update());
            entry = {box, form};
            forms.set(kind.id, entry);
        }
        return entry;
    }

    // request is what PreviewPage and ExportPage get for the chosen kind and
    // paper: the shared settings that apply to the kind and its options
    // (its defaults when it has no form), or undefined until they load.
    function request() {
        const kind = currentKind();
        const paper = currentPaper();
        if (!kind || !paper) return undefined;
        const form = formOf(kind).form;
        // Every page kind uses the font and Colour-in.
        const {font, colourIn} = shared.lettering.options();
        const page = sheet.PageSettings.createFrom({
            paper: paper.id,
            cut: kind.usesCut ? shared.cut.value : '',
            title: kind.usesTitle ? shared.text.value.trim() : '',
            font,
            colourIn,
        });
        const options = form ? JSON.stringify(form.options()) : kind.defaults || '';
        return {kind, paper, settings: page, options, key: JSON.stringify([kind.id, page, options])};
    }

    // update shows the chosen kind's form and the shared fields it uses,
    // shapes the preview like the paper and previews the options.
    function update() {
        const kind = currentKind();
        ui.kindTitle.textContent = kind ? label('pageKind', kind.id, kind.label) : '';
        const shown = kind ? formOf(kind).box : undefined;
        for (const {box} of forms.values()) box.hidden = box !== shown;
        shared.cutField.hidden = !(kind?.usesCut ?? false);
        shared.showText(kind?.usesTitle ?? false);
        syncFieldsets(ui.options);
        const paper = currentPaper();
        if (paper) ui.sheet.style.setProperty('--page-ratio', String(paper.width / paper.height));
        const req = request();
        // The preview ignores options it already shows or is rendering.
        if (req) preview.schedule({key: req.key, render: () => PreviewPage(req.kind.id, req.settings, req.options)});
        ui.exportPdf.disabled = exporting || !req;
    }

    async function exportPage() {
        const req = request();
        if (!req || exporting) return;
        deps.hideError();
        const previous = {
            text: ui.status.textContent ?? '',
            title: ui.status.title,
            open: !ui.openPdf.hidden,
            export: lastExport,
        };
        exporting = true;
        ui.exportPdf.disabled = true;
        lastExport = undefined;
        ui.status.textContent = t('print.exporting');
        ui.status.title = '';
        ui.openPdf.hidden = true;
        try {
            const saved = await ExportPage(req.kind.id, deps.defaultDir(), req.settings, req.options);
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
            // The page count of the preview, if it shows these options.
            const shown = preview.result();
            lastExport = {file: baseName(saved), pages: shown?.key === req.key ? shown.pages : undefined};
            renderExportStatus();
            ui.status.title = saved;
            ui.openPdf.hidden = false;
        } catch (err) {
            ui.status.textContent = '';
            ui.status.title = '';
            deps.showError(t('print.errExport', {error: errorText(err)}));
        } finally {
            exporting = false;
            update();
        }
    }

    // renderExportStatus shows the last saved PDF, e.g. "Saved x.pdf: 1
    // page", in the current language.
    function renderExportStatus() {
        if (!lastExport) return;
        const {file, pages} = lastExport;
        ui.status.textContent = pages === undefined
            ? t('pages.savedFile', {file})
            : t('pages.saved', {file, pages: tn('print.pages', pages)});
    }

    async function openPdf() {
        try {
            await OpenFile(lastPdf);
        } catch (err) {
            deps.showError(errorText(err));
        }
    }

    // showKinds shows the page kinds the view shows, named in the current
    // language, and checks the first of wanted that is shown, else the
    // shown default.
    function showKinds(wanted: ReadonlyArray<string | undefined>) {
        kindGroup.replaceOptions(visibleItems(view, 'pageKind', kinds)
            .map((k) => ({value: k.id, label: label('pageKind', k.id, k.label)})));
        kindGroup.value = chooseShown(view, 'pageKind', kinds, wanted);
    }

    // applyView shows the lists as view says (see PagesTab.applyView).
    function applyView(next: View) {
        const key = JSON.stringify(next ?? null);
        if (key === viewKey) return;
        view = next;
        viewKey = key;
        showKinds([kindGroup.value, saved.kind]);
        shared.setView(view, {paper: saved.paper, cut: saved.cut});
        update();
    }

    // relabel shows the tab's dynamic text in the current language, keeping
    // the choices.
    function relabel() {
        showKinds([kindGroup.value]);
        shared.relabel();
        const kind = currentKind();
        ui.kindTitle.textContent = kind ? label('pageKind', kind.id, kind.label) : '';
        preview.relabel();
        if (exporting) ui.status.textContent = t('print.exporting');
        else renderExportStatus();
    }

    async function load() {
        const [kindList, paperList, cutList, fontList] =
            await Promise.allSettled([PageKinds(), PrintPapers(), PrintCutModes(), letteringFonts()]);
        if (kindList.status === 'fulfilled') {
            kinds = kindList.value ?? [];
            // The remembered kind, or the shown default when it is hidden
            // or no longer offered.
            showKinds([saved.kind]);
        } else {
            deps.showError(t('pages.errKinds', {error: errorText(kindList.reason)}));
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
        if (fontList.status === 'fulfilled') {
            shared.lettering.setFonts(fontList.value);
        } else {
            deps.showError(t('lettering.errFonts', {error: errorText(fontList.reason)}));
        }
        shared.restore({
            paper: saved.paper,
            cut: saved.cut,
            text: saved.title,
            font: saved.font,
            colourIn: saved.colourIn,
        });
        update();
        saver.start();
    }

    shared.cut.addEventListener('change', changed);
    // A title may change the page count of some kinds; it is remembered
    // once edited (change), not on every keystroke.
    shared.text.addEventListener('input', update);
    shared.text.addEventListener('change', saver.save);
    ui.exportPdf.addEventListener('click', exportPage);
    ui.openPdf.addEventListener('click', openPdf);
    onLangChange(relabel);
    update();
    load();

    return {applyView};
}

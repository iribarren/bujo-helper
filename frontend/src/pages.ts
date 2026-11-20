// The "Make pages" tab: generated pages that need no images. Pick a page
// kind (from PageKinds), set its own options (its form, see pageForms.ts)
// and the shared Page and Lettering options, see how many pages it makes,
// and export it to a PDF (see internal/sheet.BuildPage). The preview is a
// page-shaped placeholder in the paper's proportions.

import {ExportPage, OpenFile, PageKinds, PagePages, PrintCutModes, PrintPapers} from '../wailsjs/go/main/App';
import {main, sheet} from '../wailsjs/go/models';
import {renderSharedFieldsets, syncFieldsets} from './fieldsets';
import {label, onLangChange, t, tn} from './i18n';
import {pageForms, type PageForm} from './pageForms';
import {initRadioGroup} from './radioGroup';

export interface PagesDeps {
    // The folder the save dialog starts in.
    defaultDir: () => string;
    showError: (message: string) => void;
    hideError: () => void;
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

export function initPages(deps: PagesDeps): void {
    const ui = {
        kindTitle: byId<HTMLHeadingElement>('pages-kind-title'),
        forms: byId<HTMLDivElement>('pages-form'),
        options: byId<HTMLElement>('pages-options'),
        count: byId<HTMLSpanElement>('pages-count'),
        sheet: byId<HTMLDivElement>('pages-sheet'),
        status: byId<HTMLSpanElement>('pages-status'),
        openPdf: byId<HTMLButtonElement>('pages-open-pdf'),
        exportPdf: byId<HTMLButtonElement>('pages-export'),
    };
    // The page kinds, a radio group; the user's choice calls update.
    const kindGroup = initRadioGroup(byId<HTMLDivElement>('pages-kinds'), () => update());
    // Page (paper, cut mode) and Lettering (the title), as in Print covers,
    // without a credits page: generated pages have no images to credit.
    const shared = renderSharedFieldsets({
        prefix: 'pages',
        container: ui.options,
        paperTitle: 'pages.paperTitle',
        cutTitle: 'pages.cutTitle',
        text: {id: 'title', label: 'pages.title', title: 'pages.titleTitle'},
        hint: 'pages.noTitle',
        onPaperChange: () => update(),
    });

    // The page kinds, papers and cut modes from Go; empty until they load.
    let kinds: main.PageKindInfo[] = [];
    let papers: main.PrintPaperInfo[] = [];
    // The forms built so far, by kind ID; each keeps its options while
    // another kind is shown.
    const forms = new Map<string, {box: HTMLElement; form?: PageForm}>();
    let exporting = false;
    let lastPdf = '';
    // The last saved PDF's status, to show it again in another language.
    let lastExport: {file: string; pages?: number} | undefined;
    // The page count from PagePages for the request it answers; countKey is
    // the latest request and countSeq increases with every request, so a
    // slow, older answer cannot overwrite a newer one.
    let count: {key: string; pages: number} | undefined;
    let countKey = '';
    let countSeq = 0;

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

    // request is what PagePages and ExportPage get for the chosen kind and
    // paper: the shared settings that apply to the kind and its options
    // (its defaults when it has no form), or undefined until they load.
    function request() {
        const kind = currentKind();
        const paper = currentPaper();
        if (!kind || !paper) return undefined;
        const form = formOf(kind).form;
        const settings = sheet.PageSettings.createFrom({
            paper: paper.id,
            cut: kind.usesCut ? shared.cut.value : '',
            title: kind.usesTitle ? shared.text.value.trim() : '',
        });
        const options = form ? JSON.stringify(form.options()) : kind.defaults || '';
        return {kind, paper, settings, options, key: JSON.stringify([kind.id, settings, options])};
    }

    // update shows the chosen kind's form and the shared fields it uses,
    // shapes the preview like the paper and counts the pages.
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
        countPages();
        ui.exportPdf.disabled = exporting || !request();
    }

    // countPages asks PagePages how many pages the request makes, unless
    // that is the latest request already. On failure no count is shown, as
    // the last one counted other settings.
    async function countPages() {
        const req = request();
        if (!req || req.key === countKey) return;
        countKey = req.key;
        const seq = ++countSeq;
        try {
            const pages = await PagePages(req.kind.id, req.settings, req.options);
            if (seq !== countSeq) return;
            count = {key: req.key, pages};
            renderCount();
        } catch (err) {
            if (seq !== countSeq) return;
            // Forget the key so the same request can be retried.
            countKey = '';
            count = undefined;
            renderCount();
            console.warn(`Could not count pages: ${errorText(err)}`);
        }
    }

    // renderCount shows "Page 1 of N" (the preview shows the first page).
    function renderCount() {
        ui.count.textContent = count ? t('pages.pageOf', {page: 1, total: count.pages}) : '';
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
            lastExport = {file: baseName(saved), pages: count?.key === req.key ? count.pages : undefined};
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

    // labelKinds names the page kinds in the current language.
    function labelKinds() {
        kindGroup.replaceOptions(kinds.map((k) => ({value: k.id, label: label('pageKind', k.id, k.label)})));
    }

    // relabel shows the tab's dynamic text in the current language, keeping
    // the choices.
    function relabel() {
        labelKinds();
        shared.relabel();
        const kind = currentKind();
        ui.kindTitle.textContent = kind ? label('pageKind', kind.id, kind.label) : '';
        renderCount();
        if (exporting) ui.status.textContent = t('print.exporting');
        else renderExportStatus();
    }

    async function load() {
        const [kindList, paperList, cutList] = await Promise.allSettled([PageKinds(), PrintPapers(), PrintCutModes()]);
        if (kindList.status === 'fulfilled') {
            kinds = kindList.value ?? [];
            labelKinds();
            const preset = kinds.find((k) => k.default);
            if (preset) kindGroup.value = preset.id;
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
        update();
    }

    shared.cut.addEventListener('change', update);
    // A title may change the page count of some kinds.
    shared.text.addEventListener('input', update);
    ui.exportPdf.addEventListener('click', exportPage);
    ui.openPdf.addEventListener('click', openPdf);
    onLangChange(relabel);
    update();
    load();
}

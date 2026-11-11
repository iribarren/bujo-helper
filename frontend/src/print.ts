// The "Print sheet" tab: preview the covers in a folder, select some and
// export them to an A4 PDF (see internal/sheet).

import {ChooseFolder, ExportPDF, ListCovers, OpenFile} from '../wailsjs/go/main/App';
import {main} from '../wailsjs/go/models';

// Covers per PDF page. Keep in sync with sheet.PerPage.
const PER_PAGE = 25;

export interface PrintDeps {
    // The download output folder; the print sheet shows it until the user
    // picks another folder.
    outputFolder: () => string;
    showError: (message: string) => void;
    hideError: () => void;
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

function plural(n: number, word: string): string {
    return `${n} ${word}${n === 1 ? '' : 's'}`;
}

export function pageCount(covers: number): number {
    return Math.ceil(covers / PER_PAGE);
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

    function currentDir(): string {
        return chosenDir || deps.outputFolder();
    }

    function updateSelection() {
        const n = selected.size;
        ui.selection.textContent = `${n} selected · ${plural(pageCount(n), 'page')}`;
        ui.exportPdf.disabled = n === 0 || exporting;
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
        if (paths.length === 0 || exporting) return;
        deps.hideError();
        const previous = {text: ui.status.textContent ?? '', title: ui.status.title, open: !ui.openPdf.hidden};
        exporting = true;
        updateSelection();
        ui.status.textContent = 'Exporting…';
        ui.status.title = '';
        ui.openPdf.hidden = true;
        try {
            const saved = await ExportPDF(paths, currentDir());
            if (saved === '') {
                // Dialog cancelled: leave the last result as it was.
                ui.status.textContent = previous.text;
                ui.status.title = previous.title;
                ui.openPdf.hidden = !previous.open;
                return;
            }
            lastPdf = saved;
            ui.status.textContent =
                `Saved ${baseName(saved)}: ${plural(paths.length, 'cover')} on ${plural(pageCount(paths.length), 'page')}`;
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
    updateSelection();

    return {
        activate: () => {
            load();
        },
    };
}

// The Sections part of the Settings dialog: a checkbox tree of the app
// sections (SECTIONS), each with its lists and one checkbox per item, as Go
// lists them, so sections and items added later show up here on their own.
// An optional section has a checkbox that hides it; unticking it keeps the
// item choices and greys out its lists. Print is always shown. Each list has
// an all/none checkbox, and keeps at least one item ticked: the last ticked
// one cannot be unticked, and "none" leaves the list's default.

import {settings} from '../wailsjs/go/models';
import {label, onLangChange, t} from './i18n';
import {
    SECTIONS,
    defaultItem,
    hiddenIds,
    sectionShown,
    visibleItems,
    type View,
    type ViewItem,
    type ViewList,
    type ViewSection,
} from './view';

export interface ViewConfig {
    // load fetches the items of the lists not loaded yet; a list that
    // fails says so, and the next load tries it again.
    load(): Promise<void>;
    // fill ticks what view shows (everything when it is undefined), also
    // in the lists that load later.
    fill(view: View): void;
    // read is the view the boxes show: the unticked optional sections and,
    // per list, the unticked item IDs; empty entries are left out. A list
    // that has not loaded keeps the IDs hidden in the filled view.
    read(): settings.ViewPrefs;
}

interface SectionPart {
    section: ViewSection;
    node: HTMLFieldSetElement;
    toggle: HTMLInputElement;
    lists: ListPart[];
}

interface ListPart {
    list: ViewList;
    part: SectionPart;
    node: HTMLFieldSetElement;
    master: HTMLInputElement;
    body: HTMLDivElement;
    // The loaded items, with their checkboxes; undefined until loaded.
    items?: Array<{item: ViewItem; box: HTMLInputElement}>;
    loading?: Promise<void>;
    // The "could not load" note, while the list has not loaded.
    note?: HTMLParagraphElement;
}

// check is a <label> holding a checkbox and the span with its text.
function check(className: string): {label: HTMLLabelElement; box: HTMLInputElement; text: HTMLSpanElement} {
    const node = document.createElement('label');
    node.className = `view-check ${className}`.trim();
    const box = document.createElement('input');
    box.type = 'checkbox';
    const text = document.createElement('span');
    node.append(box, text);
    return {label: node, box, text};
}

// initViewConfig builds the tree into container; onChange is called when
// the user changes a box.
export function initViewConfig(container: HTMLElement, onChange: () => void): ViewConfig {
    let view: View;
    const relabels: Array<() => void> = [];
    const lists: ListPart[] = [];

    // refresh sets the disabled and all/none states of a list's boxes.
    function refresh(part: ListPart) {
        const off = !part.part.toggle.checked;
        const items = part.items ?? [];
        const ticked = items.filter(({box}) => box.checked).length;
        for (const {box} of items) box.disabled = off || (box.checked && ticked === 1);
        part.master.checked = items.length > 0 && ticked === items.length;
        part.master.indeterminate = ticked > 0 && ticked < items.length;
        part.master.disabled = off || items.length <= 1;
    }

    function refreshSection(part: SectionPart) {
        part.node.classList.toggle('off', !part.toggle.checked);
        for (const list of part.lists) refresh(list);
    }

    // tick ticks the boxes of a list, when loaded, for the filled view.
    function tick(part: ListPart) {
        if (part.items) {
            const shown = visibleItems(view, part.list.id, part.items.map(({item}) => item));
            for (const {item, box} of part.items) box.checked = shown.includes(item);
        }
        refresh(part);
    }

    function listPart(list: ViewList, part: SectionPart): ListPart {
        const node = document.createElement('fieldset');
        node.className = 'view-list';
        node.dataset.list = list.id;
        const legend = document.createElement('legend');
        const head = check('view-list-title');
        legend.append(head.label);
        const body = document.createElement('div');
        body.className = 'view-items';
        node.append(legend, body);
        const self: ListPart = {list, part, node, master: head.box, body};

        // A click on a fully ticked list's box unticks it: "none", which
        // keeps the default; on a partly ticked one, it ticks every item.
        head.box.addEventListener('change', () => {
            const items = self.items ?? [];
            const keep = defaultItem(items.map(({item}) => item));
            for (const {item, box} of items) box.checked = head.box.checked || item === keep;
            refresh(self);
            onChange();
        });
        relabels.push(() => {
            head.text.textContent = t(list.title);
            if (self.note) self.note.textContent = t('settings.listUnavailable');
        });
        return self;
    }

    function sectionPart(section: ViewSection): SectionPart {
        const node = document.createElement('fieldset');
        node.className = 'view-section';
        node.dataset.section = section.id;
        const legend = document.createElement('legend');
        const head = check('view-section-title');
        legend.append(head.label);
        const body = document.createElement('div');
        body.className = 'view-lists';
        node.append(legend, body);
        head.box.checked = true;
        const part: SectionPart = {section, node, toggle: head.box, lists: []};

        if (section.optional) {
            head.box.addEventListener('change', () => {
                refreshSection(part);
                onChange();
            });
        } else {
            head.box.disabled = true;
            const note = document.createElement('span');
            note.className = 'view-note';
            legend.append(' ', note);
            relabels.push(() => {
                note.textContent = t('settings.alwaysShown');
            });
        }
        relabels.push(() => {
            head.text.textContent = t(section.title);
        });
        for (const list of section.lists) {
            const child = listPart(list, part);
            part.lists.push(child);
            lists.push(child);
            body.append(child.node);
        }
        return part;
    }

    // show puts a loaded list's items in, as checkboxes.
    function show(part: ListPart, items: ViewItem[]) {
        const rows = items.map((item) => {
            const row = check('view-item');
            row.box.dataset.item = item.id;
            row.box.addEventListener('change', () => {
                refresh(part);
                onChange();
            });
            const relabel = () => {
                row.text.textContent = label(part.list.id, item.id, item.label);
            };
            relabels.push(relabel);
            relabel();
            return {item, box: row.box, label: row.label};
        });
        part.items = rows.map(({item, box}) => ({item, box}));
        part.note = undefined;
        part.body.replaceChildren(...rows.map((row) => row.label));
        tick(part);
    }

    // fail says a list could not load.
    function fail(part: ListPart) {
        part.note = document.createElement('p');
        part.note.className = 'view-note';
        part.note.textContent = t('settings.listUnavailable');
        part.body.replaceChildren(part.note);
        refresh(part);
    }

    function loadList(part: ListPart): Promise<void> {
        if (part.items) return Promise.resolve();
        part.loading ??= part.list.load().then((items) => {
            if (!Array.isArray(items)) throw new Error(`no ${part.list.id} list`);
            show(part, items);
        }).catch(() => fail(part)).finally(() => {
            part.loading = undefined;
        });
        return part.loading;
    }

    const sections = SECTIONS.map(sectionPart);
    container.replaceChildren(...sections.map((part) => part.node));
    const relabel = () => {
        for (const fn of relabels) fn();
    };
    relabel();
    onLangChange(relabel);
    for (const part of sections) refreshSection(part);

    return {
        load: async () => {
            await Promise.all(lists.map(loadList));
        },
        fill(next) {
            view = next;
            for (const part of sections) {
                part.toggle.checked = sectionShown(view, part.section.id);
                part.node.classList.toggle('off', !part.toggle.checked);
            }
            for (const part of lists) tick(part);
        },
        read() {
            const hiddenSections = sections
                .filter((part) => part.section.optional && !part.toggle.checked)
                .map((part) => part.section.id);
            const hidden: Record<string, string[]> = {};
            for (const part of lists) {
                const ids = part.items
                    ? part.items.filter(({box}) => !box.checked).map(({item}) => item.id)
                    : hiddenIds(view, part.list.id);
                if (ids.length > 0) hidden[part.list.id] = [...ids];
            }
            return new settings.ViewPrefs({
                hiddenSections: hiddenSections.length > 0 ? hiddenSections : undefined,
                hidden: Object.keys(hidden).length > 0 ? hidden : undefined,
            });
        },
    };
}

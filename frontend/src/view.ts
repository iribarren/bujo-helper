// View: the parts of the app the user can hide in Settings. SECTIONS is the
// one registry of them: every tab in the tab list is a section here (a test
// guards this), with the lists inside it the user picks what to see from.
// Settings save what is hidden, not what is shown, so items Go adds to a
// list later show up ticked; a new section or list only takes an entry here.

import {
    MediaTypes,
    PageKinds,
    PrintCropModes,
    PrintCutModes,
    PrintFormats,
    PrintPapers,
    PrintSizes,
} from '../wailsjs/go/main/App';
import type {settings} from '../wailsjs/go/models';
import type {MessageKey} from './i18n/en';

// An item of a list, as Go sends it; default marks the list's default
// (the first item when none does).
export interface ViewItem {
    id: string;
    label: string;
    default?: boolean;
}

export interface ViewList {
    // id names the list in the saved settings and is the label kind of its
    // items (see i18n label).
    id: string;
    title: MessageKey;
    load: () => Promise<ViewItem[]>;
}

export interface ViewSection {
    // id is the tab's data-tab.
    id: string;
    title: MessageKey;
    // optional sections can be hidden; the others are always shown.
    optional: boolean;
    lists: readonly ViewList[];
}

// SECTIONS are the app sections in tab order. Print is always shown; its
// Paper and Cut lists also apply to Make pages, which shares them.
export const SECTIONS: readonly ViewSection[] = [
    {
        id: 'download', title: 'app.tabDownload', optional: true,
        lists: [{id: 'media', title: 'view.media', load: MediaTypes}],
    },
    {
        id: 'print', title: 'app.tabPrint', optional: false,
        lists: [
            {id: 'paper', title: 'view.paper', load: PrintPapers},
            {id: 'format', title: 'view.format', load: PrintFormats},
            {id: 'size', title: 'view.size', load: PrintSizes},
            {id: 'crop', title: 'view.crop', load: PrintCropModes},
            {id: 'cut', title: 'view.cut', load: PrintCutModes},
        ],
    },
    {
        id: 'pages', title: 'app.tabPages', optional: true,
        lists: [{id: 'pageKind', title: 'view.pageKind', load: PageKinds}],
    },
];

export type View = settings.ViewPrefs | undefined;

// sectionShown is whether the section with that id is shown: always for a
// section that is not optional or not in SECTIONS.
export function sectionShown(view: View, id: string): boolean {
    const section = SECTIONS.find((s) => s.id === id);
    if (!section?.optional) return true;
    return !(view?.hiddenSections ?? []).includes(id);
}

// hiddenIds are the IDs hidden in the list with that id.
export function hiddenIds(view: View, listId: string): string[] {
    return view?.hidden?.[listId] ?? [];
}

// defaultItem is the item marked default, else the first; undefined when
// there are no items.
export function defaultItem<T extends ViewItem>(items: readonly T[]): T | undefined {
    return items.find((item) => item.default) ?? items[0];
}

// visibleItems are the items of the list with that id the user shows, in
// order. A list never ends up empty: when every item is hidden (an old or
// hand-edited settings file), it keeps the default.
export function visibleItems<T extends ViewItem>(view: View, listId: string, items: readonly T[]): T[] {
    const hidden = hiddenIds(view, listId);
    const shown = items.filter((item) => !hidden.includes(item.id));
    if (shown.length > 0) return shown;
    const fallback = defaultItem(items);
    return fallback ? [fallback] : [];
}

// shownDefault is the item to pick when the chosen one is hidden: the
// default when it is shown, else the first shown item.
export function shownDefault<T extends ViewItem>(view: View, listId: string, items: readonly T[]): T | undefined {
    const shown = visibleItems(view, listId, items);
    return shown.find((item) => item.default) ?? shown[0];
}

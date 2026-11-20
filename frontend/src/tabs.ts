// Tabs: the sections of the window, read from the markup. Each [role=tab]
// in the tab list names itself with data-tab and its panel with
// aria-controls, so adding a section only takes markup; the tab labels are
// translated like any other static text (data-i18n).

export interface Tabs {
    // The tab names, in order.
    names: () => string[];
    // Shows the tab with that name; focus moves keyboard focus to it.
    select: (name: string, focus?: boolean) => void;
}

interface Tab {
    name: string;
    tab: HTMLElement;
    panel: HTMLElement;
}

// The tab each navigation key moves to, from tab i of n.
const KEYS: Record<string, (i: number, n: number) => number> = {
    ArrowRight: (i, n) => (i + 1) % n,
    ArrowLeft: (i, n) => (i - 1 + n) % n,
    Home: () => 0,
    End: (_i, n) => n - 1,
};

// initTabs wires the tabs of tablist: a click or the arrow keys (wrapping),
// Home and End select a tab, with a roving tab stop; onSelect gets the name
// of every tab selected.
export function initTabs(tablist: HTMLElement, onSelect: (name: string) => void): Tabs {
    const tabs: Tab[] = Array.from(tablist.querySelectorAll<HTMLElement>('[role="tab"]')).map((tab) => {
        const id = tab.getAttribute('aria-controls') ?? '';
        const panel = document.getElementById(id);
        if (!panel) throw new Error(`missing #${id}`);
        return {name: tab.dataset.tab ?? '', tab, panel};
    });

    function selectIndex(index: number, focus = false) {
        tabs.forEach(({tab, panel}, i) => {
            const on = i === index;
            tab.setAttribute('aria-selected', String(on));
            tab.tabIndex = on ? 0 : -1;
            panel.hidden = !on;
        });
        if (focus) tabs[index].tab.focus();
        onSelect(tabs[index].name);
    }

    tabs.forEach(({tab}, i) => {
        tab.addEventListener('click', () => selectIndex(i));
        tab.addEventListener('keydown', (e) => {
            const move = KEYS[e.key];
            if (!move) return;
            e.preventDefault();
            selectIndex(move(i, tabs.length), true);
        });
    });

    return {
        names: () => tabs.map((t) => t.name),
        select: (name, focus = false) => {
            const index = tabs.findIndex((t) => t.name === name);
            if (index >= 0) selectIndex(index, focus);
        },
    };
}

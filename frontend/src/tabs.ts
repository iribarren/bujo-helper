// Tabs: the sections of the window, read from the markup. Each [role=tab]
// in the tab list names itself with data-tab and its panel with
// aria-controls, so adding a section only takes markup; the tab labels are
// translated like any other static text (data-i18n).

export interface Tabs {
    // The tab names, in order.
    names: () => string[];
    // Shows the tab with that name, unless it is hidden; focus moves
    // keyboard focus to it.
    select: (name: string, focus?: boolean) => void;
    // Hides the tabs with those names and shows the others. When the
    // selected tab is hidden, the first tab shown is selected.
    setHidden: (names: readonly string[]) => void;
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
// Home and End select a tab, with a roving tab stop, skipping hidden tabs;
// onSelect gets the name of every tab selected.
export function initTabs(tablist: HTMLElement, onSelect: (name: string) => void): Tabs {
    const tabs: Tab[] = Array.from(tablist.querySelectorAll<HTMLElement>('[role="tab"]')).map((tab) => {
        const id = tab.getAttribute('aria-controls') ?? '';
        const panel = document.getElementById(id);
        if (!panel) throw new Error(`missing #${id}`);
        return {name: tab.dataset.tab ?? '', tab, panel};
    });
    // The selected tab: the one the markup selects, else the first.
    let selected = Math.max(0, tabs.findIndex(({tab}) => tab.getAttribute('aria-selected') === 'true'));

    // shown are the indexes of the tabs not hidden, in order.
    function shown(): number[] {
        return tabs.flatMap(({tab}, i) => (tab.hidden ? [] : [i]));
    }

    function selectIndex(index: number, focus = false) {
        selected = index;
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
            const order = shown();
            const at = order.indexOf(i);
            if (at >= 0) selectIndex(order[move(at, order.length)], true);
        });
    });

    return {
        names: () => tabs.map((t) => t.name),
        select: (name, focus = false) => {
            const index = tabs.findIndex((t) => t.name === name);
            if (index >= 0 && !tabs[index].tab.hidden) selectIndex(index, focus);
        },
        setHidden: (names) => {
            for (const {name, tab} of tabs) tab.hidden = names.includes(name);
            const first = shown()[0];
            if (tabs[selected]?.tab.hidden && first !== undefined) selectIndex(first);
        },
    };
}

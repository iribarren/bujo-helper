// The colour theme: the theme setting ("" is System, "light" or "dark")
// becomes the data-theme attribute of the root element, which style.css
// reads. Without the attribute the system setting (prefers-color-scheme)
// decides.

export function applyTheme(theme: string | undefined): void {
    const root = document.documentElement;
    if (theme === 'light' || theme === 'dark') {
        root.dataset.theme = theme;
    } else {
        delete root.dataset.theme;
    }
}

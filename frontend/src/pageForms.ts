// The option forms of the page kinds in the Make pages tab, by kind ID. A
// form renders the kind's own options (the shared Page and Lettering
// options are the tab's) and returns them as the object Go decodes into the
// kind's options struct (see internal/sheet.PageKinds). A new kind adds its
// form here; a kind without one is exported with its defaults.

import {titled, translated} from './fieldsets';
import {initRadioGroup} from './radioGroup';

export interface PageForm {
    // The kind's options, sent to Go as JSON.
    options: () => Record<string, unknown>;
}

// PageFormBuilder renders a kind's form into container, starting from the
// kind's defaults (from PageKinds); idPrefix prefixes its element IDs, and
// onChange is called when the user changes an option.
export type PageFormBuilder = (
    container: HTMLElement,
    idPrefix: string,
    defaults: Record<string, unknown>,
    onChange: () => void,
) => PageForm;

// The dot spacings a dot grid offers, in mm (Go: sheet.DotSpacings), and
// the one it starts with when Go's default is not one of them.
const DOT_SPACINGS = [4, 5, 6];
const DEFAULT_DOT_SPACING = 5;

// dotGridForm is the dot grid's form: Dot spacing, a segmented group.
const dotGridForm: PageFormBuilder = (container, idPrefix, defaults, onChange) => {
    const field = document.createElement('div');
    field.className = 'field';
    const caption = translated(document.createElement('span'), 'pages.dotSpacing');
    caption.id = `${idPrefix}-spacing-label`;
    const box = titled(document.createElement('div'), 'pages.dotSpacingTitle');
    box.id = `${idPrefix}-spacing`;
    box.className = 'segmented';
    box.setAttribute('role', 'radiogroup');
    box.setAttribute('aria-labelledby', caption.id);
    field.append(caption, box);
    container.append(field);

    const spacing = initRadioGroup(box, onChange);
    spacing.replaceOptions(DOT_SPACINGS.map((mm) => ({value: String(mm), label: `${mm} mm`})));
    const preset = Number(defaults.spacing);
    spacing.value = String(DOT_SPACINGS.includes(preset) ? preset : DEFAULT_DOT_SPACING);
    return {
        options: () => ({spacing: Number(spacing.value)}),
    };
};

export const pageForms: Record<string, PageFormBuilder> = {
    dotgrid: dotGridForm,
};

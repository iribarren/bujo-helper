// The plant list and editor of the plant care cards form (pageForms.ts):
// one button per plant, with a remove button, and Add plant (see cardList),
// and the selected plant's fields: Name, Light, Water, Soil, Notes and its
// image (ChoosePlantImage), a nature plate.
//
// An empty name is sent as is: Go prints the name read from the image's
// file name ("Common name - Scientific name.jpg", as nature downloads are
// named), and the plant's button shows its common name meanwhile.

import {ChoosePlantImage} from '../wailsjs/go/main/App';
import {baseName, cardList, cardTextField, imagePicker, isString} from './recipesForm';

// Plant is one plant as Go decodes it (Go: sheet.Plant).
export interface Plant {
    // The printed name; "" takes it from the image's file name.
    name: string;
    light: string;
    water: string;
    soil: string;
    notes: string;
    // The absolute path of the plate image, "" for none.
    image: string;
}

const FIELDS = ['name', 'light', 'water', 'soil', 'notes', 'image'] as const;

const blankPlant = (): Plant => ({name: '', light: '', water: '', soil: '', notes: '', image: ''});

// plantList is the plants of a kind's defaults: every object, its fields
// of the wrong type read as empty.
export function plantList(value: unknown): Plant[] {
    if (!Array.isArray(value)) return [];
    return value.filter((p) => typeof p === 'object' && p !== null).map((p) => {
        const out = blankPlant();
        for (const f of FIELDS) if (isString(p[f])) out[f] = p[f];
        return out;
    });
}

// commonName is the common name in an image's file name, as Go's
// ParseName reads it: the name before its last " - ", extension dropped.
function commonName(path: string): string {
    const stem = baseName(path).replace(/\.[^.]*$/, '');
    const i = stem.lastIndexOf(' - ');
    return (i >= 0 ? stem.slice(0, i) : stem).trim();
}

export interface PlantEditor {
    // The plants as Go decodes them, in list order.
    plants: () => Plant[];
    dispose: () => void;
}

// plantEditor appends the plant list and editor to container, with element
// IDs prefixed by idPrefix, starting with the plants initial, the first one
// selected. onChange is called when a plant is added, removed or edited;
// selecting one changes nothing.
export function plantEditor(container: HTMLElement, idPrefix: string, initial: Plant[], onChange: () => void): PlantEditor {
    const list = cardList(container, {
        idPrefix,
        className: 'plants',
        keys: {
            list: 'pages.plantsList',
            untitled: 'pages.plantsUntitled',
            remove: 'pages.plantsRemove',
            add: 'pages.plantsAdd',
            hint: 'pages.plantsHint',
        },
        initial: initial.map((p) => ({...p})),
        blank: blankPlant,
        name: (p) => p.name.trim() || (p.image ? commonName(p.image) : ''),
        render: (p) => {
            name.input.value = p.name;
            light.input.value = p.light;
            water.input.value = p.water;
            soil.input.value = p.soil;
            notes.input.value = p.notes;
            image.render();
        },
        onAdd: () => name.input.focus(),
        onChange,
    });

    const name = cardTextField(list, 'name', 'pages.plantsName', {renames: true, placeholder: 'pages.plantsNamePlaceholder'});
    const image = imagePicker(list, 'image', {
        label: 'pages.plantsImage',
        choose: 'pages.plantsImageChoose',
        none: 'pages.plantsImageNone',
        remove: 'pages.plantsImageRemove',
    }, ChoosePlantImage);
    const light = cardTextField(list, 'light', 'pages.plantsLight');
    const water = cardTextField(list, 'water', 'pages.plantsWater');
    const soil = cardTextField(list, 'soil', 'pages.plantsSoil');
    const notes = cardTextField(list, 'notes', 'pages.plantsNotes', {rows: 4});
    list.editor.append(name.field, image.field, light.field, water.field, soil.field, notes.field);
    list.select(list.cards[0]);

    return {
        plants: () => list.cards.map((p) => ({...p})),
        dispose: () => {
            list.dispose();
            image.dispose();
        },
    };
}

// A radio group of buttons (role="radio") in a role="radiogroup" container,
// used like a <select>: value, disabled and replaceable options. One radio
// is a tab stop; the arrow keys move and select (wrapping), Home and End
// jump to the first and last radio.

export interface RadioOption {
    value: string;
    label: string;
}

export interface RadioGroup {
    // The checked option's value, "" when none is. Setting it does not call
    // onChange, like a select's value.
    value: string;
    disabled: boolean;
    // Shows options, keeping the checked value when it is still there and
    // checking the first option otherwise.
    replaceOptions: (options: RadioOption[]) => void;
}

// The radio each navigation key moves to, from radio i of n.
const KEYS: Record<string, (i: number, n: number) => number> = {
    ArrowRight: (i, n) => (i + 1) % n,
    ArrowDown: (i, n) => (i + 1) % n,
    ArrowLeft: (i, n) => (i - 1 + n) % n,
    ArrowUp: (i, n) => (i - 1 + n) % n,
    Home: () => 0,
    End: (_i, n) => n - 1,
};

// initRadioGroup renders radios into container; onChange is called when the
// user checks another radio.
export function initRadioGroup(container: HTMLElement, onChange: () => void): RadioGroup {
    let radios: HTMLButtonElement[] = [];
    let value = '';
    let disabled = false;

    // check marks the radio with value v as checked and the only tab stop;
    // with no such radio, none is checked and the first is the tab stop.
    function check(v: string) {
        const found = radios.some((r) => r.dataset.value === v);
        value = found ? v : '';
        radios.forEach((radio, i) => {
            const on = radio.dataset.value === value;
            radio.setAttribute('aria-checked', String(on));
            radio.tabIndex = on || (!found && i === 0) ? 0 : -1;
        });
    }

    // pick checks the radio at index on user input, and reports a change.
    function pick(index: number, focus: boolean) {
        const radio = radios[index];
        if (focus) radio.focus();
        if (radio.dataset.value === value) return;
        check(radio.dataset.value ?? '');
        onChange();
    }

    function replaceOptions(options: RadioOption[]) {
        radios = options.map((option, i) => {
            const radio = document.createElement('button');
            radio.type = 'button';
            radio.className = 'radio-button';
            radio.setAttribute('role', 'radio');
            radio.dataset.value = option.value;
            radio.textContent = option.label;
            radio.title = option.label;
            radio.disabled = disabled;
            radio.addEventListener('click', () => pick(i, false));
            radio.addEventListener('keydown', (e) => {
                const move = KEYS[e.key];
                if (!move) return;
                e.preventDefault();
                pick(move(i, radios.length), true);
            });
            return radio;
        });
        container.replaceChildren(...radios);
        const kept = options.some((o) => o.value === value);
        check(kept ? value : options[0]?.value ?? '');
    }

    return {
        get value() {
            return value;
        },
        set value(v: string) {
            check(v);
        },
        get disabled() {
            return disabled;
        },
        set disabled(on: boolean) {
            disabled = on;
            for (const radio of radios) radio.disabled = on;
        },
        replaceOptions,
    };
}

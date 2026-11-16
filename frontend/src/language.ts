import {SetLanguage} from '../wailsjs/go/main/App';
import {detectLang, setLang, type Lang} from './i18n';

// useLanguage resolves the language setting ("" is System, which follows the
// navigator language), switches the UI to it (notifying onLangChange
// listeners when it changed) and tells Go, which translates native dialogs,
// its errors and PDF labels. A failure to tell Go is logged: the UI still
// switches.
export async function useLanguage(setting: string): Promise<Lang> {
    const lang = detectLang(setting, navigator.language || '');
    setLang(lang);
    try {
        await SetLanguage(lang);
    } catch (err) {
        console.warn(`Could not set the language in Go: ${err instanceof Error ? err.message : String(err)}`);
    }
    return lang;
}

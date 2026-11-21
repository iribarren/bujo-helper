export namespace main {
	
	export class CoverView {
	    name: string;
	    path: string;
	    thumbUrl: string;
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new CoverView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.thumbUrl = source["thumbUrl"];
	        this.width = source["width"];
	        this.height = source["height"];
	    }
	}
	export class ItemView {
	    index: number;
	    title: string;
	    creator: string;
	
	    static createFrom(source: any = {}) {
	        return new ItemView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.title = source["title"];
	        this.creator = source["creator"];
	    }
	}
	export class LetteringFontInfo {
	    id: string;
	    label: string;
	    group: string;
	    hollow: string;
	    licence: string;
	    url: string;
	    licenceUrl: string;
	    default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LetteringFontInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.group = source["group"];
	        this.hollow = source["hollow"];
	        this.licence = source["licence"];
	        this.url = source["url"];
	        this.licenceUrl = source["licenceUrl"];
	        this.default = source["default"];
	    }
	}
	export class MediaTypeInfo {
	    id: string;
	    label: string;
	    creatorLabel: string;
	    dateLabel: string;
	    sources: string[];
	
	    static createFrom(source: any = {}) {
	        return new MediaTypeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.creatorLabel = source["creatorLabel"];
	        this.dateLabel = source["dateLabel"];
	        this.sources = source["sources"];
	    }
	}
	export class PageKindInfo {
	    id: string;
	    label: string;
	    usesCut: boolean;
	    usesTitle: boolean;
	    defaults: string;
	    default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PageKindInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.usesCut = source["usesCut"];
	        this.usesTitle = source["usesTitle"];
	        this.defaults = source["defaults"];
	        this.default = source["default"];
	    }
	}
	export class PreviewInfo {
	    url: string;
	    pages: number;
	
	    static createFrom(source: any = {}) {
	        return new PreviewInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.pages = source["pages"];
	    }
	}
	export class PrintCropModeInfo {
	    id: string;
	    label: string;
	    usesSize: boolean;
	    usesCut: boolean;
	    frames: number;
	    default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrintCropModeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.usesSize = source["usesSize"];
	        this.usesCut = source["usesCut"];
	        this.frames = source["frames"];
	        this.default = source["default"];
	    }
	}
	export class PrintCutModeInfo {
	    id: string;
	    label: string;
	    default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrintCutModeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.default = source["default"];
	    }
	}
	export class PrintFormatInfo {
	    id: string;
	    label: string;
	    noun: string;
	    usesSize: boolean;
	    usesCut: boolean;
	    usesCrop: boolean;
	    usesHeader: boolean;
	    usesLettering: boolean;
	    default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrintFormatInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.noun = source["noun"];
	        this.usesSize = source["usesSize"];
	        this.usesCut = source["usesCut"];
	        this.usesCrop = source["usesCrop"];
	        this.usesHeader = source["usesHeader"];
	        this.usesLettering = source["usesLettering"];
	        this.default = source["default"];
	    }
	}
	export class PrintPagesInfo {
	    pages: number;
	    allSquare: boolean;
	    exact: boolean;
	    widePerPage: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new PrintPagesInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pages = source["pages"];
	        this.allSquare = source["allSquare"];
	        this.exact = source["exact"];
	        this.widePerPage = source["widePerPage"];
	    }
	}
	export class PrintPaperInfo {
	    id: string;
	    label: string;
	    width: number;
	    height: number;
	    default: boolean;
	    perPage: Record<string, any>;
	    headerPerPage: Record<string, number>;
	    squarePerPage: Record<string, any>;
	    cropPerPage: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new PrintPaperInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.default = source["default"];
	        this.perPage = source["perPage"];
	        this.headerPerPage = source["headerPerPage"];
	        this.squarePerPage = source["squarePerPage"];
	        this.cropPerPage = source["cropPerPage"];
	    }
	}
	export class PrintSizeInfo {
	    id: string;
	    label: string;
	    coverHeight: number;
	    default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrintSizeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.coverHeight = source["coverHeight"];
	        this.default = source["default"];
	    }
	}
	export class StartRequest {
	    mediaType: string;
	    text: string;
	    outputDir: string;
	    country: string;
	
	    static createFrom(source: any = {}) {
	        return new StartRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mediaType = source["mediaType"];
	        this.text = source["text"];
	        this.outputDir = source["outputDir"];
	        this.country = source["country"];
	    }
	}

}

export namespace quotes {
	
	export class Quote {
	    text: string;
	    author: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new Quote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.author = source["author"];
	        this.source = source["source"];
	    }
	}
	export class Result {
	    person: string;
	    url: string;
	    matches: string[];
	    quotes: Quote[];
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.person = source["person"];
	        this.url = source["url"];
	        this.matches = source["matches"];
	        this.quotes = this.convertValues(source["quotes"], Quote);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace settings {
	
	export class Settings {
	    tmdbApiKey: string;
	    igdbClientId: string;
	    igdbClientSecret: string;
	    bhlApiKey: string;
	    language: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tmdbApiKey = source["tmdbApiKey"];
	        this.igdbClientId = source["igdbClientId"];
	        this.igdbClientSecret = source["igdbClientSecret"];
	        this.bhlApiKey = source["bhlApiKey"];
	        this.language = source["language"];
	    }
	}

}

export namespace sheet {
	
	export class Labels {
	    Admit: string;
	    Date: string;
	    Where: string;
	    CardDate: string;
	    Credits: string;
	    Fonts: string;
	
	    static createFrom(source: any = {}) {
	        return new Labels(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Admit = source["Admit"];
	        this.Date = source["Date"];
	        this.Where = source["Where"];
	        this.CardDate = source["CardDate"];
	        this.Credits = source["Credits"];
	        this.Fonts = source["Fonts"];
	    }
	}
	export class Options {
	    format: string;
	    size: string;
	    paper: string;
	    cut: string;
	    crop: string;
	    dateLabel: string;
	    header: string;
	    font: string;
	    colourIn: boolean;
	    credits: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Options(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.size = source["size"];
	        this.paper = source["paper"];
	        this.cut = source["cut"];
	        this.crop = source["crop"];
	        this.dateLabel = source["dateLabel"];
	        this.header = source["header"];
	        this.font = source["font"];
	        this.colourIn = source["colourIn"];
	        this.credits = source["credits"];
	    }
	}
	export class PageLabels {
	    Months: string[];
	    Weekdays: string[];
	    NewMoon: string;
	    FullMoon: string;
	    MapTitle: string;
	    MapVisited: string;
	    HabitsTitle: string;
	    Language: string;
	
	    static createFrom(source: any = {}) {
	        return new PageLabels(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Months = source["Months"];
	        this.Weekdays = source["Weekdays"];
	        this.NewMoon = source["NewMoon"];
	        this.FullMoon = source["FullMoon"];
	        this.MapTitle = source["MapTitle"];
	        this.MapVisited = source["MapVisited"];
	        this.HabitsTitle = source["HabitsTitle"];
	        this.Language = source["Language"];
	    }
	}
	export class PageSettings {
	    paper: string;
	    cut: string;
	    title: string;
	    font: string;
	    colourIn: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PageSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.paper = source["paper"];
	        this.cut = source["cut"];
	        this.title = source["title"];
	        this.font = source["font"];
	        this.colourIn = source["colourIn"];
	    }
	}

}


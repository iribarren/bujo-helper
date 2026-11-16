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
	export class MediaTypeInfo {
	    id: string;
	    label: string;
	    creatorLabel: string;
	    dateLabel: string;
	
	    static createFrom(source: any = {}) {
	        return new MediaTypeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.creatorLabel = source["creatorLabel"];
	        this.dateLabel = source["dateLabel"];
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
	    usesHeader: boolean;
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
	        this.usesHeader = source["usesHeader"];
	        this.default = source["default"];
	    }
	}
	export class PrintPagesInfo {
	    pages: number;
	    allSquare: boolean;
	    exact: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrintPagesInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pages = source["pages"];
	        this.allSquare = source["allSquare"];
	        this.exact = source["exact"];
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

export namespace settings {
	
	export class Settings {
	    tmdbApiKey: string;
	    igdbClientId: string;
	    igdbClientSecret: string;
	    language: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tmdbApiKey = source["tmdbApiKey"];
	        this.igdbClientId = source["igdbClientId"];
	        this.igdbClientSecret = source["igdbClientSecret"];
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
	
	    static createFrom(source: any = {}) {
	        return new Labels(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Admit = source["Admit"];
	        this.Date = source["Date"];
	        this.Where = source["Where"];
	        this.CardDate = source["CardDate"];
	    }
	}
	export class Options {
	    format: string;
	    size: string;
	    paper: string;
	    cut: string;
	    dateLabel: string;
	    header: string;
	
	    static createFrom(source: any = {}) {
	        return new Options(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.format = source["format"];
	        this.size = source["size"];
	        this.paper = source["paper"];
	        this.cut = source["cut"];
	        this.dateLabel = source["dateLabel"];
	        this.header = source["header"];
	    }
	}

}


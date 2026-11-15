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
	
	    static createFrom(source: any = {}) {
	        return new MediaTypeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.creatorLabel = source["creatorLabel"];
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
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tmdbApiKey = source["tmdbApiKey"];
	    }
	}

}


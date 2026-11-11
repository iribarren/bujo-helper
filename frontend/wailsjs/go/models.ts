export namespace main {
	
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


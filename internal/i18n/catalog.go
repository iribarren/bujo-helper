package i18n

// catalogs holds the messages by language, then key. Every English key
// must exist in every other language with the same fmt verbs, in the same
// order; a test checks it. Errors are wrapped at the call site, so
// messages never use %w.
var catalogs = map[Lang]map[string]string{
	EN: {
		// Native dialogs.
		"dialog.importCSV":          "Import CSV",
		"dialog.saveSampleCSV":      "Save sample CSV",
		"dialog.chooseOutputFolder": "Choose output folder",
		"dialog.exportPDF":          "Export PDF",
		"filter.csv":                "CSV files (*.csv)",
		"filter.all":                "All files (*.*)",
		"filter.pdf":                "PDF files (*.pdf)",

		// The prompt before replacing an existing file.
		"replace.title":   "Replace file?",
		"replace.message": "%s already exists. Replace it?",
		"replace.confirm": "Replace",
		"replace.cancel":  "Cancel",

		// Errors returned to the UI.
		"err.noItemsInFile":      "%s: no items found",
		"err.unknownMediaType":   "unknown media type %q",
		"err.noItems":            `no items: enter one "Title, Creator" per line`,
		"err.chooseOutputFolder": "choose an output folder",
		"err.createOutputFolder": "create output folder",
		"err.downloadRunning":    "a download is already running",
		"err.noFolderToOpen":     "no folder to open",
		"err.noFileToOpen":       "no file to open",
		"err.chooseFolder":       "choose a folder",
		"err.selectCover":        "select at least one cover",
		"err.settingsLocation":   "save settings: the settings file location is unknown",

		// PDF labels.
		"pdf.stubAdmit": "ADMIT ONE",
		"pdf.stubDate":  "Date:",
		"pdf.stubWhere": "Where:",
		"pdf.cardDate":  "Date",
		"pdf.credits":   "Credits",
		"pdf.fonts":     "Fonts",

		// Generated page labels. Months and weekdays are comma-separated
		// lists: the 12 month names from January, and the 7 short weekday
		// names from Monday.
		"pdf.months":   "January,February,March,April,May,June,July,August,September,October,November,December",
		"pdf.weekdays": "Mon,Tue,Wed,Thu,Fri,Sat,Sun",
		"pdf.moonNew":  "New",
		"pdf.moonFull": "Full",
	},
	ES: {
		"dialog.importCSV":          "Importar CSV",
		"dialog.saveSampleCSV":      "Guardar CSV de ejemplo",
		"dialog.chooseOutputFolder": "Elegir carpeta de destino",
		"dialog.exportPDF":          "Exportar PDF",
		"filter.csv":                "Archivos CSV (*.csv)",
		"filter.all":                "Todos los archivos (*.*)",
		"filter.pdf":                "Archivos PDF (*.pdf)",

		"replace.title":   "¿Reemplazar el archivo?",
		"replace.message": "%s ya existe. ¿Desea reemplazarlo?",
		"replace.confirm": "Reemplazar",
		"replace.cancel":  "Cancelar",

		"err.noItemsInFile":      "%s: no se han encontrado elementos",
		"err.unknownMediaType":   "tipo de medio desconocido: %q",
		"err.noItems":            `no hay elementos: escriba uno por línea con el formato "Título, Autor"`,
		"err.chooseOutputFolder": "elija una carpeta de destino",
		"err.createOutputFolder": "no se ha podido crear la carpeta de destino",
		"err.downloadRunning":    "ya hay una descarga en curso",
		"err.noFolderToOpen":     "no hay ninguna carpeta que abrir",
		"err.noFileToOpen":       "no hay ningún archivo que abrir",
		"err.chooseFolder":       "elija una carpeta",
		"err.selectCover":        "seleccione al menos una portada",
		"err.settingsLocation":   "guardar ajustes: se desconoce la ubicación del archivo de ajustes",

		"pdf.stubAdmit": "ENTRADA",
		"pdf.stubDate":  "Fecha:",
		"pdf.stubWhere": "Lugar:",
		"pdf.cardDate":  "Fecha",
		"pdf.credits":   "Créditos",
		"pdf.fonts":     "Fuentes",

		"pdf.months":   "Enero,Febrero,Marzo,Abril,Mayo,Junio,Julio,Agosto,Septiembre,Octubre,Noviembre,Diciembre",
		"pdf.weekdays": "Lun,Mar,Mié,Jue,Vie,Sáb,Dom",
		"pdf.moonNew":  "Nueva",
		"pdf.moonFull": "Llena",
	},
}

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
		"dialog.importRecipes":      "Import recipes",
		"dialog.recipePhoto":        "Choose recipe photo",
		"filter.markdown":           "Markdown files (*.md, *.markdown, *.txt)",
		"filter.images":             "Images (*.jpg, *.jpeg, *.png)",

		// The prompt before replacing an existing file.
		"replace.title":   "Replace file?",
		"replace.message": "%s already exists. Replace it?",
		"replace.confirm": "Replace",
		"replace.cancel":  "Cancel",

		// Errors returned to the UI.
		"err.noItemsInFile":      "%s: no items found",
		"err.noRecipesInFile":    "%s: no recipes found",
		"err.unknownMediaType":   "unknown media type %q",
		"err.noItems":            `no items: enter one "Title, Creator" per line`,
		"err.chooseOutputFolder": "choose an output folder",
		"err.createOutputFolder": "create output folder",
		"err.downloadRunning":    "a download is already running",
		"err.noFolderToOpen":     "no folder to open",
		"err.noFileToOpen":       "no file to open",
		"err.notPDF":             "%s is not a PDF",
		"err.chooseFolder":       "choose a folder",
		"err.selectCover":        "select at least one cover",
		"err.settingsLocation":   "save settings: the settings file location is unknown",
		// {query} is replaced with the searched name (not a fmt verb).
		"err.noQuotes":   `no quotes found for "{query}"`,
		"err.quoteQuery": "enter a name to search for quotes",

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
		// The visited map's default title and the caption of its country
		// list.
		"pdf.mapTitle":   "Places I've been",
		"pdf.mapVisited": "Countries visited",
		// The habit tracker's default title: "{month}" is replaced by the
		// month name (it is not a format string).
		"pdf.habitsTitle": "Habits · {month}",
		// The quote cards' default title and the caption of pages holding
		// a Wikiquote quote.
		"pdf.quotesTitle":  "Words to keep",
		"pdf.quotesCredit": "Quotes from Wikiquote (CC BY-SA 4.0)",
		// Recipe cards: the column headings, the words starting the meta
		// line ("Serves 4 · Time 30 min") and the mark of continuation
		// cards, printed in brackets after the title.
		"pdf.recipeIngredients": "Ingredients",
		"pdf.recipeSteps":       "Steps",
		"pdf.recipeServes":      "Serves",
		"pdf.recipeTime":        "Time",
		"pdf.recipeContinued":   "continued",
		// Kitchen conversions: the default title, the table headings, the
		// unit words (abbreviations as cookbooks write them), the gas mark
		// column, the oven table's caption and the cups-to-grams
		// ingredients, a comma-separated list of 10.
		"pdf.conversionsTitle":      "Kitchen conversions",
		"pdf.conversionVolume":      "Volume",
		"pdf.conversionWeight":      "Weight",
		"pdf.conversionOven":        "Oven temperatures",
		"pdf.conversionCupsToGrams": "Cups to grams",
		"pdf.conversionCup":         "cup",
		"pdf.conversionTbsp":        "tbsp",
		"pdf.conversionTsp":         "tsp",
		"pdf.conversionGasMark":     "Gas mark",
		"pdf.conversionFanOven":     "Fan oven: 20 °C lower",
		"pdf.conversionIngredients": "Flour,Sugar,Brown sugar,Icing sugar,Butter,Rice,Rolled oats,Honey,Cocoa powder,Milk or water",
	},
	ES: {
		"dialog.importCSV":          "Importar CSV",
		"dialog.saveSampleCSV":      "Guardar CSV de ejemplo",
		"dialog.chooseOutputFolder": "Elegir carpeta de destino",
		"dialog.exportPDF":          "Exportar PDF",
		"filter.csv":                "Archivos CSV (*.csv)",
		"filter.all":                "Todos los archivos (*.*)",
		"filter.pdf":                "Archivos PDF (*.pdf)",
		"dialog.importRecipes":      "Importar recetas",
		"dialog.recipePhoto":        "Elegir foto de la receta",
		"filter.markdown":           "Archivos Markdown (*.md, *.markdown, *.txt)",
		"filter.images":             "Imágenes (*.jpg, *.jpeg, *.png)",

		"replace.title":   "¿Reemplazar el archivo?",
		"replace.message": "%s ya existe. ¿Desea reemplazarlo?",
		"replace.confirm": "Reemplazar",
		"replace.cancel":  "Cancelar",

		"err.noItemsInFile":      "%s: no se han encontrado elementos",
		"err.noRecipesInFile":    "%s: no se han encontrado recetas",
		"err.unknownMediaType":   "tipo de medio desconocido: %q",
		"err.noItems":            `no hay elementos: escriba uno por línea con el formato "Título, Autor"`,
		"err.chooseOutputFolder": "elija una carpeta de destino",
		"err.createOutputFolder": "no se ha podido crear la carpeta de destino",
		"err.downloadRunning":    "ya hay una descarga en curso",
		"err.noFolderToOpen":     "no hay ninguna carpeta que abrir",
		"err.noFileToOpen":       "no hay ningún archivo que abrir",
		"err.notPDF":             "%s no es un PDF",
		"err.chooseFolder":       "elija una carpeta",
		"err.selectCover":        "seleccione al menos una portada",
		"err.settingsLocation":   "guardar ajustes: se desconoce la ubicación del archivo de ajustes",
		"err.noQuotes":           "no se encontraron citas de «{query}»",
		"err.quoteQuery":         "escriba un nombre para buscar citas",

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

		"pdf.mapTitle":   "Lugares que he visitado",
		"pdf.mapVisited": "Países visitados",

		"pdf.habitsTitle": "Hábitos · {month}",

		"pdf.quotesTitle":  "Palabras para guardar",
		"pdf.quotesCredit": "Citas de Wikiquote (CC BY-SA 4.0)",

		"pdf.recipeIngredients": "Ingredientes",
		"pdf.recipeSteps":       "Pasos",
		"pdf.recipeServes":      "Raciones",
		"pdf.recipeTime":        "Tiempo",
		"pdf.recipeContinued":   "continuación",

		"pdf.conversionsTitle":      "Conversiones de cocina",
		"pdf.conversionVolume":      "Volumen",
		"pdf.conversionWeight":      "Peso",
		"pdf.conversionOven":        "Temperaturas del horno",
		"pdf.conversionCupsToGrams": "Tazas a gramos",
		"pdf.conversionCup":         "taza",
		"pdf.conversionTbsp":        "cda.",
		"pdf.conversionTsp":         "cdta.",
		"pdf.conversionGasMark":     "Gas",
		"pdf.conversionFanOven":     "Horno con ventilador: 20 °C menos",
		"pdf.conversionIngredients": "Harina,Azúcar,Azúcar moreno,Azúcar glas,Mantequilla,Arroz,Copos de avena,Miel,Cacao en polvo,Leche o agua",
	},
}

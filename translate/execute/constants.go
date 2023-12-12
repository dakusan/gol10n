//Constants used in automations
//go:build !gol10n_read_compiled_only

package execute

//goland:noinspection GoSnakeCaseUsage
const (
	SettingsFileName      = "settings-gol10n.json"
	VarDictionaryFileBase = "variables"
	YAML_Extension        = "yaml"
	JSON_Extension        = "json"
)

//goland:noinspection GoSnakeCaseUsage,GoCommentStart
const (
	_ ProcessedFileFlag = 1 << iota

	//Language object info
	PFF_Language_SuccessfullyLoaded   //If the Language object was successfully loaded and filled into ProcessedFiles and the fallback was set
	PFF_Language_SuccessNoFallbackSet //If the Language object was loaded and filled into ProcessedFiles, but the fallback was not set
	PFF_Language_IsDefault            //If this is the default language

	//Loading state (mutually exclusive)
	PFF_Load_NotAttempted //File loading was not attempted because other errors occurred first
	PFF_Load_NotFound     //File was not loaded because its translation text file was not found
	PFF_Load_YAML         //If this was loaded from a YAML translation text file
	PFF_Load_JSON         //If this was loaded from a JSON translation text file
	PFF_Load_Compiled     //If this was loaded from a .gtr file (compression state is assumed from ProcessSettings.CompressCompiled)

	//Error information
	PFF_Error_DuringProcessing //If errors occurred during processing

	//File output success flags
	PFF_OutputSuccess_CompiledLanguage   //If a .gtr file was successfully output (only when ProcessSettings.OutputCompiled, compression state is assumed from ProcessSettings.CompressCompiled)
	PFF_OutputSuccess_CompiledDictionary //If a .gtr dictionary file was successfully output (only when ProcessSettings.OutputCompiled and PFF_Language_IsDefault, compression state is assumed from ProcessSettings.CompressCompiled)
	PFF_OutputSuccess_GoDictionaries     //If one or more go dictionary files was successfully output (only when ProcessSettings.OutputGoDictionary and PFF_Language_IsDefault)
)

// ProcessedFileFlagName : See ProcessedFileFlagNames
type ProcessedFileFlagName struct {
	Flag      ProcessedFileFlag
	Name      string
	ShortName [4]byte //All shortname strings must be 4 bytes
}

// ProcessedFileFlagNames is named information about the ProcessedFileFlags
var ProcessedFileFlagNames = []ProcessedFileFlagName{
	createPFFN(1, "UNUSED", "    "),
	createPFFN(PFF_Language_SuccessfullyLoaded, "Language_SuccessfullyLoaded", "SuLD"),
	createPFFN(PFF_Language_SuccessNoFallbackSet, "Language_SuccessNoFallbackSet", "SuNF"),
	createPFFN(PFF_Language_IsDefault, "Language_IsDefault", "Defa"),
	createPFFN(PFF_Load_NotAttempted, "Load_NotAttempted", "LoNA"),
	createPFFN(PFF_Load_NotFound, "Load_NotFound", "LoNF"),
	createPFFN(PFF_Load_YAML, "Load_YAML", "LoYA"),
	createPFFN(PFF_Load_JSON, "Load_JSON", "LoJS"),
	createPFFN(PFF_Load_Compiled, "Load_Compiled", "LoCo"),
	createPFFN(PFF_Error_DuringProcessing, "Error_DuringProcessing", "Er  "),
	createPFFN(PFF_OutputSuccess_CompiledLanguage, "OutputSuccess_CompiledLanguage", "OuCL"),
	createPFFN(PFF_OutputSuccess_CompiledDictionary, "OutputSuccess_CompiledDictionary", "OuCD"),
	createPFFN(PFF_OutputSuccess_GoDictionaries, "OutputSuccess_GoDictionaries", "OuGD"),
}

func createPFFN(Flag ProcessedFileFlag, Name string, shortName string) ProcessedFileFlagName {
	return ProcessedFileFlagName{Flag, Name, [4]byte([]byte(shortName))}
}

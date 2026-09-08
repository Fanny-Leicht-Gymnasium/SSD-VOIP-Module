package main

import "strings"

// PiperVoice describes one selectable Piper TTS voice model.
// Name matches the Piper/Hugging Face model naming convention
// "<lang>_<REGION>-<speaker>-<quality>" (without the .onnx suffix).
type PiperVoice struct {
	Name        string
	Description string
}

// piperVoices is a curated list of commonly used Piper voices. It's not
// exhaustive \u2014 the full catalog lives at
// https://huggingface.co/rhasspy/piper-voices \u2014 so typing a name that
// isn't in this list is still accepted as a custom value.
var piperVoices = []PiperVoice{
	{"de_DE-thorsten-low", "German, Thorsten, low quality"},
	{"de_DE-thorsten-medium", "German, Thorsten, medium quality"},
	{"de_DE-thorsten-high", "German, Thorsten, high quality"},
	{"de_DE-thorsten_emotional-medium", "German, Thorsten (emotional), medium"},
	{"de_DE-kerstin-low", "German, Kerstin, low quality"},
	{"de_DE-eva_k-x_low", "German, Eva K, extra-low quality"},
	{"de_DE-ramona-low", "German, Ramona, low quality"},
	{"de_DE-pavoque-low", "German, Pavoque, low quality"},
	{"de_DE-karlsson-low", "German, Karlsson, low quality"},
	{"en_US-lessac-low", "English (US), Lessac, low quality"},
	{"en_US-lessac-medium", "English (US), Lessac, medium quality"},
	{"en_US-lessac-high", "English (US), Lessac, high quality"},
	{"en_US-amy-low", "English (US), Amy, low quality"},
	{"en_US-amy-medium", "English (US), Amy, medium quality"},
	{"en_US-danny-low", "English (US), Danny, low quality"},
	{"en_US-kathleen-low", "English (US), Kathleen, low quality"},
	{"en_US-ryan-low", "English (US), Ryan, low quality"},
	{"en_US-ryan-medium", "English (US), Ryan, medium quality"},
	{"en_US-ryan-high", "English (US), Ryan, high quality"},
	{"en_US-joe-medium", "English (US), Joe, medium quality"},
	{"en_GB-alan-low", "English (UK), Alan, low quality"},
	{"en_GB-alan-medium", "English (UK), Alan, medium quality"},
	{"en_GB-jenny_dioco-medium", "English (UK), Jenny (Dioco), medium"},
	{"en_GB-southern_english_female-low", "English (UK), Southern female, low"},
	{"en_GB-northern_english_male-medium", "English (UK), Northern male, medium"},
	{"fr_FR-siwis-low", "French, Siwis, low quality"},
	{"fr_FR-siwis-medium", "French, Siwis, medium quality"},
	{"fr_FR-gilles-low", "French, Gilles, low quality"},
	{"es_ES-davefx-medium", "Spanish, Davefx, medium quality"},
	{"es_ES-mls_10246-low", "Spanish, MLS 10246, low quality"},
	{"it_IT-riccardo-x_low", "Italian, Riccardo, extra-low quality"},
	{"it_IT-paola-medium", "Italian, Paola, medium quality"},
	{"nl_NL-mls_5809-low", "Dutch, MLS 5809, low quality"},
	{"nl_NL-rdh-medium", "Dutch, RDH, medium quality"},
	{"pt_BR-faber-medium", "Portuguese (BR), Faber, medium quality"},
	{"pl_PL-darkman-medium", "Polish, Darkman, medium quality"},
	{"ru_RU-irina-medium", "Russian, Irina, medium quality"},
	{"tr_TR-fahrettin-medium", "Turkish, Fahrettin, medium quality"},
	{"zh_CN-huayan-medium", "Chinese (Mandarin), Huayan, medium quality"},
}

// filterPiperVoices returns voices whose name or description contains
// query (case-insensitive). An empty query returns the full list.
func filterPiperVoices(query string) []PiperVoice {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return piperVoices
	}

	var matches []PiperVoice
	for _, v := range piperVoices {
		if strings.Contains(strings.ToLower(v.Name), query) ||
			strings.Contains(strings.ToLower(v.Description), query) {
			matches = append(matches, v)
		}
	}
	return matches
}
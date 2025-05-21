package consts

import "deepResearch/entity"

var LanguageSchema = []*entity.FieldSchema{
	{
		Name:        "langCode11",
		Type:        "string",
		Description: "ISO 639-1 language code",
		MaxLength:   10,
		Required:    true,
	},
	{
		Name:        "languageStyle",
		Type:        "string",
		Description: "[vibe & tone] in [what language], such as formal english, informal chinese, technical german, humor english, slang, genZ, emojis etc.",
		MaxLength:   100,
		Required:    true,
	},
}

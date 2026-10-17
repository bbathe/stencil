package renderer

// styleName maps Markdown element keys to built-in Word style names.
func styleName(key string) string {
	switch key {
	case "heading1":
		return "Heading 1"
	case "heading2":
		return "Heading 2"
	case "heading3":
		return "Heading 3"
	case "heading4":
		return "Heading 4"
	case "heading5":
		return "Heading 5"
	case "heading6":
		return "Heading 6"
	case "body":
		return "Normal"
	case "code_block":
		return "Code"
	case "blockquote":
		return "Quote"
	case "list_item":
		return "List Paragraph"
	default:
		return "Normal"
	}
}

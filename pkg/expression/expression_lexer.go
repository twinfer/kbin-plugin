package expression

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// ExpressionTokenType defines the types of tokens specific to Kaitai expressions.
type ExpressionTokenType int

const (
	EXPR_ILLEGAL ExpressionTokenType = iota
	EXPR_EOF
	EXPR_WS // Whitespace

	// Literals
	EXPR_IDENT   // myVar, _root, _
	EXPR_NUMBER  // 123, 0xABC, 1.5e-2
	EXPR_STRING  // "hello", 'world'
	EXPR_BOOLEAN // true, false
	EXPR_NULL    // null

	// Operators
	EXPR_PLUS             // +
	EXPR_MINUS            // -
	EXPR_STAR             // *
	EXPR_SLASH            // /
	EXPR_MOD              // %
	EXPR_EQ               // ==
	EXPR_NEQ              // !=
	EXPR_LT               // <
	EXPR_GT               // >
	EXPR_LE               // <=
	EXPR_GE               // >=
	EXPR_LOGIC_AND        // &&
	EXPR_LOGIC_OR         // ||
	EXPR_LOGIC_NOT        // !
	EXPR_BIT_AND          // &
	EXPR_BIT_OR           // |
	EXPR_BIT_XOR          // ^
	EXPR_BIT_NOT          // ~
	EXPR_LSHIFT           // <<
	EXPR_RSHIFT           // >>
	EXPR_TERNARY_QUESTION // ?
	EXPR_TERNARY_COLON    // :

	// Delimiters
	EXPR_LPAREN   // (
	EXPR_RPAREN   // )
	EXPR_LBRACKET // [
	EXPR_RBRACKET // ]
	EXPR_DOT      // .
	EXPR_COMMA    // ,

	// Built-in functions/keywords in expressions
	EXPR_SIZEOF  // sizeof
	EXPR_ALIGNOF // alignof
	EXPR_AS      // as (for type casting like `.as<type>()`)

	// Special identifiers (handled as keywords for direct token mapping)
	EXPR_SELF            // _
	EXPR_IO              // _io
	EXPR_PARENT          // _parent
	EXPR_ROOT            // _root
	EXPR_BYTES_REMAINING // _bytes_remaining
)

var exprKeywords = map[string]ExpressionTokenType{
	"true":             EXPR_BOOLEAN,
	"false":            EXPR_BOOLEAN,
	"null":             EXPR_NULL,
	"sizeof":           EXPR_SIZEOF,
	"alignof":          EXPR_ALIGNOF,
	"as":               EXPR_AS,
	"_":                EXPR_SELF,
	"_io":              EXPR_IO,
	"_parent":          EXPR_PARENT,
	"_root":            EXPR_ROOT,
	"_bytes_remaining": EXPR_BYTES_REMAINING,
}

// ExpressionToken represents a lexical token for expressions.
type ExpressionToken struct {
	Type    ExpressionTokenType
	Literal string
	Line    int
	Column  int
}

func (t ExpressionToken) String() string {
	if t.Type == EXPR_IDENT || t.Type == EXPR_NUMBER || t.Type == EXPR_STRING {
		return fmt.Sprintf("%s(%q) at %d:%d", t.Type.String(), t.Literal, t.Line, t.Column)
	}
	return fmt.Sprintf("%s at %d:%d", t.Type.String(), t.Line, t.Column)
}

// ExpressionTokenType String representation (for debugging)
func (t ExpressionTokenType) String() string {
	switch t {
	case EXPR_ILLEGAL:
		return "ILLEGAL"
	case EXPR_EOF:
		return "EOF"
	case EXPR_WS:
		return "WS"
	case EXPR_IDENT:
		return "IDENT"
	case EXPR_NUMBER:
		return "NUMBER"
	case EXPR_STRING:
		return "STRING"
	case EXPR_BOOLEAN:
		return "BOOLEAN"
	case EXPR_NULL:
		return "NULL"

	case EXPR_PLUS:
		return "+"
	case EXPR_MINUS:
		return "-"
	case EXPR_STAR:
		return "*"
	case EXPR_SLASH:
		return "/"
	case EXPR_MOD:
		return "%"
	case EXPR_EQ:
		return "=="
	case EXPR_NEQ:
		return "!="
	case EXPR_LT:
		return "<"
	case EXPR_GT:
		return ">"
	case EXPR_LE:
		return "<="
	case EXPR_GE:
		return ">="
	case EXPR_LOGIC_AND:
		return "&&"
	case EXPR_LOGIC_OR:
		return "||"
	case EXPR_LOGIC_NOT:
		return "!"
	case EXPR_BIT_AND:
		return "&"
	case EXPR_BIT_OR:
		return "|"
	case EXPR_BIT_XOR:
		return "^"
	case EXPR_BIT_NOT:
		return "~"
	case EXPR_LSHIFT:
		return "<<"
	case EXPR_RSHIFT:
		return ">>"
	case EXPR_TERNARY_QUESTION:
		return "?"
	case EXPR_TERNARY_COLON:
		return ":"

	case EXPR_LPAREN:
		return "("
	case EXPR_RPAREN:
		return ")"
	case EXPR_LBRACKET:
		return "["
	case EXPR_RBRACKET:
		return "]"
	case EXPR_DOT:
		return "."
	case EXPR_COMMA:
		return ","

	case EXPR_SIZEOF:
		return "SIZEOF"
	case EXPR_ALIGNOF:
		return "ALIGNOF"
	case EXPR_AS:
		return "AS"
	case EXPR_SELF:
		return "SELF"
	case EXPR_IO:
		return "IO"
	case EXPR_PARENT:
		return "PARENT"
	case EXPR_ROOT:
		return "ROOT"
	case EXPR_BYTES_REMAINING:
		return "BYTES_REMAINING"
	default:
		return fmt.Sprintf("UNKNOWN_EXPR_TOKEN(%d)", t)
	}
}

// ExpressionLexer scans the input expression string for tokens.
type ExpressionLexer struct {
	reader *bufio.Reader
	input  string // original input string
	line   int
	column int  // column of the current character (1-indexed)
	ch     rune // current character
}

// NewExpressionLexer creates a new ExpressionLexer.
func NewExpressionLexer(r io.Reader) *ExpressionLexer {
	// If it's a string reader, capture the original input
	var input string
	if sr, ok := r.(*strings.Reader); ok {
		// Get the original string from the reader
		size := sr.Size()
		buf := make([]byte, size)
		sr.Seek(0, io.SeekStart)
		sr.Read(buf)
		sr.Seek(0, io.SeekStart) // Reset position
		input = string(buf)
	}

	l := &ExpressionLexer{
		reader: bufio.NewReader(r),
		input:  input,
		line:   1,
		column: 1,
	}
	l.readChar() // Read the first character
	return l
}

// GetInput returns the original input string if available
func (l *ExpressionLexer) GetInput() string {
	return l.input
}

// readChar reads the next character and advances the position.
func (l *ExpressionLexer) readChar() {
	if l.ch != 0 { // If not first read or EOF
		if l.ch == '\n' {
			l.line++
			l.column = 1
		} else {
			l.column++
		}
	}

	r, _, err := l.reader.ReadRune()
	if err != nil {
		l.ch = 0 // EOF character
		return
	}

	l.ch = r // Set the current character
}

// peekChar returns the next character without advancing.
func (l *ExpressionLexer) peekChar() rune {
	r, _, err := l.reader.ReadRune()
	if err != nil {
		return 0 // EOF
	}
	l.reader.UnreadRune()
	return r
}

// NextToken scans and returns the next token.
func (l *ExpressionLexer) NextToken() ExpressionToken {
	l.skipWhitespace()

	// Store the current position
	tok := ExpressionToken{
		Line:   l.line,
		Column: l.column,
	}

	switch l.ch {
	case '+':
		tok.Type = EXPR_PLUS
		tok.Literal = "+"
	case '-':
		tok.Type = EXPR_MINUS
		tok.Literal = "-"
	case '*':
		tok.Type = EXPR_STAR
		tok.Literal = "*"
	case '/':
		tok.Type = EXPR_SLASH
		tok.Literal = "/"
	case '%':
		tok.Type = EXPR_MOD
		tok.Literal = "%"
	case '=':
		if l.peekChar() == '=' {
			startCol := l.column
			l.readChar() // Consume second '='
			tok.Type = EXPR_EQ
			tok.Literal = "=="
			tok.Column = startCol // Token starts at first '='
		} else {
			tok.Type = EXPR_ILLEGAL // Assignment not allowed in Kaitai expr
			tok.Literal = "="
		}
	case '!':
		if l.peekChar() == '=' {
			startCol := l.column
			l.readChar() // Consume '='
			tok.Type = EXPR_NEQ
			tok.Literal = "!="
			tok.Column = startCol
		} else {
			tok.Type = EXPR_LOGIC_NOT
			tok.Literal = "!"
		}
	case '<':
		if l.peekChar() == '=' {
			startCol := l.column
			l.readChar() // Consume '='
			tok.Type = EXPR_LE
			tok.Literal = "<="
			tok.Column = startCol
		} else if l.peekChar() == '<' {
			startCol := l.column
			l.readChar() // Consume second '<'
			tok.Type = EXPR_LSHIFT
			tok.Literal = "<<"
			tok.Column = startCol
		} else {
			tok.Type = EXPR_LT
			tok.Literal = "<"
		}
	case '>':
		if l.peekChar() == '=' {
			startCol := l.column
			l.readChar() // Consume '='
			tok.Type = EXPR_GE
			tok.Literal = ">="
			tok.Column = startCol
		} else if l.peekChar() == '>' {
			startCol := l.column
			l.readChar() // Consume second '>'
			tok.Type = EXPR_RSHIFT
			tok.Literal = ">>"
			tok.Column = startCol
		} else {
			tok.Type = EXPR_GT
			tok.Literal = ">"
		}
	case '&':
		if l.peekChar() == '&' {
			startCol := l.column
			l.readChar() // Consume second '&'
			tok.Type = EXPR_LOGIC_AND
			tok.Literal = "&&"
			tok.Column = startCol
		} else {
			tok.Type = EXPR_BIT_AND
			tok.Literal = "&"
		}
	case '|':
		if l.peekChar() == '|' {
			startCol := l.column
			l.readChar() // Consume second '|'
			tok.Type = EXPR_LOGIC_OR
			tok.Literal = "||"
			tok.Column = startCol
		} else {
			tok.Type = EXPR_BIT_OR
			tok.Literal = "|"
		}
	case '^':
		tok.Type = EXPR_BIT_XOR
		tok.Literal = "^"
	case '~':
		tok.Type = EXPR_BIT_NOT
		tok.Literal = "~"
	case '?':
		tok.Type = EXPR_TERNARY_QUESTION
		tok.Literal = "?"
	case ':':
		tok.Type = EXPR_TERNARY_COLON
		tok.Literal = ":"
	case '(':
		tok.Type = EXPR_LPAREN
		tok.Literal = "("
	case ')':
		tok.Type = EXPR_RPAREN
		tok.Literal = ")"
	case '[':
		tok.Type = EXPR_LBRACKET
		tok.Literal = "["
	case ']':
		tok.Type = EXPR_RBRACKET
		tok.Literal = "]"
	case '.':
		tok.Type = EXPR_DOT
		tok.Literal = "."
	case ',':
		tok.Type = EXPR_COMMA
		tok.Literal = ","
	case '"', '\'':
		tok.Type = EXPR_STRING
		tok.Literal = l.readString(l.ch)
		// readString already consumes the closing quote and advances l.ch appropriately
		return tok
	case 0: // EOF
		tok.Type = EXPR_EOF
		tok.Literal = "" // Empty literal for EOF token
		// Keep column position where it was after last token
		return tok
	default:
		if isExprIdentifierStart(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = lookupExprKeyword(tok.Literal)
			return tok // readIdentifier already advanced l.ch
		} else if unicode.IsDigit(l.ch) {
			tok.Literal = l.readNumber()
			tok.Type = EXPR_NUMBER
			return tok // readNumber already advanced l.ch
		} else {
			tok.Type = EXPR_ILLEGAL
			tok.Literal = string(l.ch)
		}
	}

	l.readChar() // Consume the current character (for single-char tokens and the first char of multi-char operators/illegal)
	return tok
}

func (l *ExpressionLexer) skipWhitespace() {
	for unicode.IsSpace(l.ch) {
		if l.ch == '\n' {
			l.line++
			l.column = 1
		}
		r, _, err := l.reader.ReadRune()
		if err != nil {
			l.ch = 0
			return
		}
		l.ch = r
		if l.ch != '\n' {
			l.column++
		}
	}
}

// readIdentifier reads an identifier token.
func (l *ExpressionLexer) readIdentifier() string {
	var sb strings.Builder
	for isExprIdentifierPart(l.ch) {
		sb.WriteRune(l.ch)
		l.readChar()
	}
	return sb.String()
}

// readNumber reads a number literal.
func (l *ExpressionLexer) readNumber() string {
	var sb strings.Builder
	// Handle prefixes like 0x, 0o, 0b
	if l.ch == '0' {
		sb.WriteRune(l.ch)
		l.readChar()
		if l.ch == 'x' || l.ch == 'X' {
			sb.WriteRune(l.ch)
			l.readChar()
			for isHexDigit(l.ch) {
				sb.WriteRune(l.ch)
				l.readChar()
			}
			return sb.String()
		} else if l.ch == 'o' || l.ch == 'O' {
			sb.WriteRune(l.ch)
			l.readChar()
			for isOctalDigit(l.ch) {
				sb.WriteRune(l.ch)
				l.readChar()
			}
			return sb.String()
		} else if l.ch == 'b' || l.ch == 'B' {
			sb.WriteRune(l.ch)
			l.readChar()
			for isBinaryDigit(l.ch) {
				sb.WriteRune(l.ch)
				l.readChar()
			}
			return sb.String()
		}
		// If it was just '0' followed by a non-prefix, treat as decimal
	}

	// Read decimal part
	for unicode.IsDigit(l.ch) {
		sb.WriteRune(l.ch)
		l.readChar()
	}

	// Read fractional part
	if l.ch == '.' && unicode.IsDigit(l.peekChar()) {
		sb.WriteRune(l.ch)
		l.readChar()
		for unicode.IsDigit(l.ch) {
			sb.WriteRune(l.ch)
			l.readChar()
		}
	}

	// Read exponent part
	if l.ch == 'e' || l.ch == 'E' {
		sb.WriteRune(l.ch)
		l.readChar()
		if l.ch == '+' || l.ch == '-' {
			sb.WriteRune(l.ch)
			l.readChar()
		}
		for unicode.IsDigit(l.ch) {
			sb.WriteRune(l.ch)
			l.readChar()
		}
	}

	return sb.String()
}

// readString reads a string literal, handling escapes.
func (l *ExpressionLexer) readString(quoteChar rune) string {
	var sb strings.Builder
	l.readChar() // Consume the opening quote

	for l.ch != 0 && l.ch != quoteChar {
		if l.ch == '\\' {
			l.readChar() // Consume '\'
			switch l.ch {
			case 'n':
				sb.WriteRune('\n')
				l.readChar()
			case 'r':
				sb.WriteRune('\r')
				l.readChar()
			case 't':
				sb.WriteRune('\t')
				l.readChar()
			case '\\':
				sb.WriteRune('\\')
				l.readChar()
			case '"':
				sb.WriteRune('"')
				l.readChar()
			case '\'':
				sb.WriteRune('\'')
				l.readChar()
			case 'u': // Unicode escape \uXXXX
				l.readChar() // Consume 'u'
				hexDigits := make([]rune, 4)
				for i := 0; i < 4; i++ {
					if !isHexDigit(l.ch) {
						return "" // Invalid unicode escape
					}
					hexDigits[i] = l.ch
					l.readChar()
				}
				val, err := strconv.ParseInt(string(hexDigits), 16, 32)
				if err != nil {
					return "" // Invalid unicode escape
				}
				sb.WriteRune(rune(val))
			default:
				sb.WriteRune('\\')
				sb.WriteRune(l.ch)
				l.readChar()
			}
		} else {
			sb.WriteRune(l.ch)
			l.readChar()
		}
	}

	if l.ch == quoteChar {
		l.readChar() // Consume the closing quote
	}
	return sb.String()
}

func isExprIdentifierStart(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isExprIdentifierPart(ch rune) bool {
	return unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_'
}

func isHexDigit(ch rune) bool {
	return unicode.IsDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func isOctalDigit(ch rune) bool {
	return ch >= '0' && ch <= '7'
}

func isBinaryDigit(ch rune) bool {
	return ch == '0' || ch == '1'
}

// lookupExprKeyword checks if an identifier is a keyword.
func lookupExprKeyword(ident string) ExpressionTokenType {
	if tok, ok := exprKeywords[ident]; ok {
		return tok
	}
	return EXPR_IDENT
}

// Example usage of the expression parser:
/*
import "strings"

func main() {
	expression := "_io.pos + (2 * 3) == _parent.id.as<u4>()"
	lexer := NewExpressionLexer(strings.NewReader(expression))
	parser := NewExpressionParser(lexer)

	ast, err := parser.Parse()
	if err != nil {
		fmt.Printf("Error parsing expression: %v\n", err)
		for _, e := range parser.Errors() {
			fmt.Println(e)
		}
		return
	}
	fmt.Printf("Parsed AST: %s\n", ast.String()) // Using the String() method on AST nodes
}
*/

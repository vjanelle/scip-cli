package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	mdrenderer "github.com/gomarkdown/markdown/md"
	"github.com/gomarkdown/markdown/parser"
)

// markdownDocument is the CLI's presentation-oriented Markdown model. Command
// renderers shape data into this structure first, then a shared AST builder
// turns it into gomarkdown nodes for final rendering.
type markdownDocument struct {
	Title    string
	Sections []markdownSection
}

// markdownSection groups related list items under one heading.
type markdownSection struct {
	Heading string
	Items   []markdownListItem
}

// markdownListItem models one bullet with optional nested detail bullets.
type markdownListItem struct {
	Text     string
	Children []string
}

// renderMarkdownDocument translates the intermediate document model into a
// gomarkdown AST and then lets gomarkdown's Markdown renderer handle spacing,
// escaping, and list formatting.
func renderMarkdownDocument(stdout io.Writer, document markdownDocument) error {
	rendered := markdown.Render(buildMarkdownAST(document), mdrenderer.NewRenderer())
	_, err := stdout.Write(rendered)
	return err
}

// buildMarkdownAST maps the lightweight CLI document model onto the richer
// gomarkdown AST that the upstream Markdown renderer expects.
func buildMarkdownAST(document markdownDocument) ast.Node {
	root := &ast.Document{}
	appendHeading(root, 1, document.Title)
	for _, section := range document.Sections {
		appendMarkdownSection(root, section)
	}
	return root
}

func appendMarkdownSection(parent ast.Node, section markdownSection) {
	if section.Heading != "" {
		appendHeading(parent, 2, section.Heading)
	}
	if len(section.Items) == 0 {
		return
	}

	list := &ast.List{BulletChar: '-'}
	for _, item := range section.Items {
		listItem := &ast.ListItem{BulletChar: '-'}
		appendParsedBlocks(listItem, item.Text)
		for _, child := range item.Children {
			appendNestedListItem(listItem, child)
		}
		ast.AppendChild(list, listItem)
	}
	ast.AppendChild(parent, list)
}

func appendNestedListItem(parent ast.Node, text string) {
	var nested *ast.List
	children := parent.GetChildren()
	if len(children) > 0 {
		// Reuse the trailing nested list when a list item already has one so the
		// rendered Markdown stays grouped under a single parent bullet.
		lastChild, ok := children[len(children)-1].(*ast.List)
		if ok {
			nested = lastChild
		}
	}
	if nested == nil {
		nested = &ast.List{BulletChar: '-'}
		ast.AppendChild(parent, nested)
	}

	item := &ast.ListItem{BulletChar: '-'}
	appendParsedBlocks(item, text)
	ast.AppendChild(nested, item)
}

func appendHeading(parent ast.Node, level int, text string) {
	heading := &ast.Heading{Level: level}
	ast.AppendChild(heading, &ast.Text{Leaf: ast.Leaf{Literal: []byte(text)}})
	ast.AppendChild(parent, heading)
}

// appendParsedBlocks lets the document model stay string-oriented while
// delegating inline parsing and escaping details to gomarkdown's parser.
func appendParsedBlocks(parent ast.Node, source string) {
	parsed := markdown.Parse([]byte(source), parser.New())
	for _, child := range parsed.GetChildren() {
		ast.AppendChild(parent, cloneMarkdownNode(child))
	}
}

// cloneMarkdownNode copies parsed nodes before re-parenting them into the
// final AST. gomarkdown's AppendChild removes a node from its original tree,
// which would otherwise drop any existing children.
func cloneMarkdownNode(node ast.Node) ast.Node {
	switch typed := node.(type) {
	case *ast.Paragraph:
		clone := &ast.Paragraph{}
		cloneMarkdownChildren(clone, typed)
		return clone
	case *ast.Text:
		return &ast.Text{Leaf: ast.Leaf{Literal: append([]byte(nil), typed.Literal...)}}
	case *ast.Code:
		return &ast.Code{Leaf: ast.Leaf{Literal: append([]byte(nil), typed.Literal...)}}
	case *ast.Emph:
		clone := &ast.Emph{}
		cloneMarkdownChildren(clone, typed)
		return clone
	case *ast.Strong:
		clone := &ast.Strong{}
		cloneMarkdownChildren(clone, typed)
		return clone
	case *ast.Link:
		clone := &ast.Link{
			Destination: append([]byte(nil), typed.Destination...),
			Title:       append([]byte(nil), typed.Title...),
		}
		cloneMarkdownChildren(clone, typed)
		return clone
	case *ast.Hardbreak:
		return &ast.Hardbreak{}
	default:
		// Fall back to a text node so unexpected inline node types still render
		// something readable instead of disappearing.
		return &ast.Text{Leaf: ast.Leaf{Literal: []byte(ast.ToString(node))}}
	}
}

func cloneMarkdownChildren(parent ast.Node, source ast.Node) {
	for _, child := range source.GetChildren() {
		ast.AppendChild(parent, cloneMarkdownNode(child))
	}
}

func markdownCode(value string) string {
	fenceWidth := longestBacktickRun(value) + 1
	fence := strings.Repeat("`", fenceWidth)

	// Pad content that touches a backtick boundary so CommonMark keeps the
	// literal text inside the code span instead of treating it as a delimiter.
	if strings.HasPrefix(value, "`") || strings.HasSuffix(value, "`") {
		value = " " + value + " "
	}

	return fmt.Sprintf("%s%s%s", fence, value, fence)
}

func markdownText(value string) string {
	replacer := strings.NewReplacer(
		"\\", "\\\\",
		"`", "\\`",
		"*", "\\*",
		"_", "\\_",
		"[", "\\[",
		"]", "\\]",
		"<", "\\<",
		">", "\\>",
	)
	return replacer.Replace(value)
}

func longestBacktickRun(value string) int {
	longest := 0
	current := 0
	for _, r := range value {
		if r == '`' {
			current++
			if current > longest {
				longest = current
			}
			continue
		}
		current = 0
	}
	return longest
}

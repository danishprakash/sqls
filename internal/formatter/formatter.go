package formatter

import (
	"github.com/lighttiger2505/sqls/ast"
	"github.com/lighttiger2505/sqls/ast/astutil"
	"github.com/lighttiger2505/sqls/internal/lsp"
	"github.com/lighttiger2505/sqls/parser"
	"github.com/lighttiger2505/sqls/token"
)

func Format(text string, params lsp.DocumentFormattingParams) ([]lsp.TextEdit, error) {
	parsed, err := parser.Parse(text)
	if err != nil {
		return nil, err
	}

	st := lsp.Position{
		Line:      parsed.Pos().Line,
		Character: parsed.Pos().Col,
	}
	en := lsp.Position{
		Line:      parsed.End().Line,
		Character: parsed.End().Col,
	}
	formatted := Eval(parsed, &formatEnvironment{})

	res := []lsp.TextEdit{
		{
			Range: lsp.Range{
				Start: st,
				End:   en,
			},
			NewText: formatted.String(),
		},
	}
	return res, nil
}

type formatEnvironment struct {
	reader      *astutil.NodeReader
	indentLevel int
}

func (e *formatEnvironment) indentLevelReset() {
	e.indentLevel = 0
}

func (e *formatEnvironment) indentLevelUp() {
	e.indentLevel++
}

func (e *formatEnvironment) indentLevelDown() {
	e.indentLevel--
}

func (e *formatEnvironment) genIndent() []ast.Node {
	nodes := []ast.Node{}
	for i := 0; i < e.indentLevel; i++ {
		nodes = append(nodes, indentNode)
	}
	return nodes
}

type prefixFormatFn func(nodes []ast.Node, reader *astutil.NodeReader, env formatEnvironment) ([]ast.Node, formatEnvironment)

type prefixFormatMap struct {
	matcher   *astutil.NodeMatcher
	formatter prefixFormatFn
}

func (pfm *prefixFormatMap) isMatch(reader *astutil.NodeReader) bool {
	if pfm.matcher != nil && reader.CurNodeIs(*pfm.matcher) {
		return true
	}
	return false
}

func Eval(node ast.Node, env *formatEnvironment) ast.Node {
	// dPrintf("eval %q: %T\n", node, node)
	switch node := node.(type) {
	// case *ast.Query:
	// 	return formatQuery(node, env)
	// case *ast.Statement:
	// 	return formatStatement(node, env)
	case *ast.Item:
		return formatItem(node, env)
	case *ast.MultiKeyword:
		return formatMultiKeyword(node, env)
	case *ast.Aliased:
		return formatAliased(node, env)
	case *ast.Identifer:
		return formatIdentifer(node, env)
	case *ast.MemberIdentifer:
		return formatMemberIdentifer(node, env)
	case *ast.Operator:
		return formatOperator(node, env)
	case *ast.Comparison:
		return formatComparison(node, env)
	case *ast.Parenthesis:
		return formatParenthesis(node, env)
	// case *ast.ParenthesisInner:
	// case *ast.FunctionLiteral:
	case *ast.IdentiferList:
		return formatIdentiferList(node, env)
	// case *ast.SwitchCase:
	// case *ast.Null:
	default:
		if list, ok := node.(ast.TokenList); ok {
			return formatTokenList(list, env)
		} else {
			return formatNode(node, env)
		}
	}
}

func formatItem(node ast.Node, env *formatEnvironment) ast.Node {
	results := []ast.Node{node}

	whitespaceAfterMatcher := astutil.NodeMatcher{
		ExpectKeyword: []string{
			"JOIN",
			"ON",
			"AND",
			"OR",
			"LIMIT",
		},
	}
	if whitespaceAfterMatcher.IsMatch(node) {
		results = append(results, whitespaceNode)
	}
	whitespaceAroundMatcher := astutil.NodeMatcher{
		ExpectKeyword: []string{
			"BETWEEN",
			"USING",
		},
	}
	if whitespaceAroundMatcher.IsMatch(node) {
		results = unshift(results, whitespaceNode)
		results = append(results, whitespaceNode)
	}

	// Add an adjustment before the cursor
	outdentBeforeMatcher := astutil.NodeMatcher{
		ExpectKeyword: []string{
			"FROM",
			"JOIN",
			"WHERE",
			"HAVING",
			"LIMIT",
			"UNION",
			"VALUES",
			"SET",
			"EXCEPT",
		},
	}
	if outdentBeforeMatcher.IsMatch(node) {
		env.indentLevelDown()
		results = unshift(results, env.genIndent()...)
		results = unshift(results, linebreakNode)
	}
	indentBeforeMatcher := astutil.NodeMatcher{
		ExpectKeyword: []string{
			"ON",
		},
	}
	if indentBeforeMatcher.IsMatch(node) {
		env.indentLevelUp()
		results = unshift(results, env.genIndent()...)
		results = unshift(results, linebreakNode)
	}
	linebreakBeforeMatcher := astutil.NodeMatcher{
		ExpectKeyword: []string{
			"AND",
			"OR",
		},
	}
	if linebreakBeforeMatcher.IsMatch(node) {
		results = unshift(results, env.genIndent()...)
		results = unshift(results, linebreakNode)
	}

	// Add an adjustment after the cursor
	indentAfterMatcher := astutil.NodeMatcher{
		ExpectKeyword: []string{
			"SELECT",
			"FROM",
			"WHERE",
		},
		ExpectTokens: []token.Kind{
			token.LParen,
		},
	}
	if indentAfterMatcher.IsMatch(node) {
		results = append(results, linebreakNode)
		env.indentLevelUp()
		results = append(results, env.genIndent()...)
	}
	linebreakAfterMatcher := astutil.NodeMatcher{
		ExpectTokens: []token.Kind{
			token.Comma,
		},
	}
	if linebreakAfterMatcher.IsMatch(node) {
		results = append(results, linebreakNode)
		results = append(results, env.genIndent()...)
	}

	return &ast.ItemWith{Toks: results}
}

func formatMultiKeyword(node ast.Node, env *formatEnvironment) ast.Node {
	results := []ast.Node{node}

	joinKeywords := []string{
		"INNER JOIN",
		"CROSS JOIN",
		"OUTER JOIN",
		"LEFT JOIN",
		"RIGHT JOIN",
		"LEFT OUTER JOIN",
		"RIGHT OUTER JOIN",
	}

	whitespaceAfterMatcher := astutil.NodeMatcher{
		ExpectKeyword: joinKeywords,
	}
	if whitespaceAfterMatcher.IsMatch(node) {
		results = append(results, whitespaceNode)
	}

	outdentBeforeMatcher := astutil.NodeMatcher{
		ExpectKeyword: joinKeywords,
	}
	if outdentBeforeMatcher.IsMatch(node) {
		env.indentLevelDown()
		results = unshift(results, env.genIndent()...)
		results = unshift(results, linebreakNode)
	}

	return &ast.ItemWith{Toks: results}
}

func formatAliased(node *ast.Aliased, env *formatEnvironment) ast.Node {
	var results []ast.Node
	if node.IsAs {
		results = []ast.Node{
			Eval(node.RealName, env),
			whitespaceNode,
			node.As,
			whitespaceNode,
			Eval(node.AliasedName, env),
		}
	} else {
		results = []ast.Node{
			Eval(node.RealName, env),
			whitespaceNode,
			Eval(node.AliasedName, env),
		}
	}
	return &ast.ItemWith{Toks: results}
}

func formatIdentifer(node ast.Node, env *formatEnvironment) ast.Node {
	results := []ast.Node{node}
	// results := []ast.Node{node, whitespaceNode}

	// commaMatcher := astutil.NodeMatcher{
	// 	ExpectTokens: []token.Kind{
	// 		token.Comma,
	// 	},
	// }
	// if !env.reader.PeekNodeIs(true, commaMatcher) {
	// 	results = append(results, whitespaceNode)
	// }

	return &ast.ItemWith{Toks: results}
}

func formatMemberIdentifer(node *ast.MemberIdentifer, env *formatEnvironment) ast.Node {
	results := []ast.Node{
		Eval(node.Parent, env),
		periodNode,
		Eval(node.Child, env),
	}
	return &ast.ItemWith{Toks: results}
}

func formatOperator(node *ast.Operator, env *formatEnvironment) ast.Node {
	results := []ast.Node{
		Eval(node.Left, env),
		whitespaceNode,
		node.Operator,
		whitespaceNode,
		Eval(node.Right, env),
	}
	return &ast.ItemWith{Toks: results}
}

func formatComparison(node *ast.Comparison, env *formatEnvironment) ast.Node {
	results := []ast.Node{
		Eval(node.Left, env),
		whitespaceNode,
		node.Comparison,
		whitespaceNode,
		Eval(node.Right, env),
	}
	return &ast.ItemWith{Toks: results}
}

func formatParenthesis(node *ast.Parenthesis, env *formatEnvironment) ast.Node {
	results := []ast.Node{}
	// results = append(results, whitespaceNode)
	results = append(results, lparenNode)
	startIndentLevel := env.indentLevel
	env.indentLevelUp()
	results = append(results, linebreakNode)
	results = append(results, env.genIndent()...)
	results = append(results, Eval(node.Inner(), env))
	env.indentLevel = startIndentLevel
	results = append(results, linebreakNode)
	results = append(results, env.genIndent()...)
	results = append(results, rparenNode)
	// results = append(results, whitespaceNode)
	return &ast.ItemWith{Toks: results}
}

func formatIdentiferList(identiferList *ast.IdentiferList, env *formatEnvironment) ast.Node {
	idents := identiferList.GetIdentifers()
	results := []ast.Node{}
	for i, ident := range idents {
		results = append(results, Eval(ident, env))
		if i != len(idents)-1 {
			results = append(results, commaNode, linebreakNode)
			results = append(results, env.genIndent()...)
		}
	}
	return &ast.ItemWith{Toks: results}
}

func formatTokenList(list ast.TokenList, env *formatEnvironment) ast.Node {
	results := []ast.Node{}
	reader := astutil.NewNodeReader(list)
	for reader.NextNode(true) {
		env.reader = reader
		results = append(results, Eval(reader.CurNode, env))
	}
	reader.Node.SetTokens(results)
	return reader.Node
}

func formatNode(node ast.Node, env *formatEnvironment) ast.Node {
	return node
}

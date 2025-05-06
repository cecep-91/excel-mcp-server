package tools

import (
	"context"
	"fmt"
	"html"
	"strings"

	z "github.com/Oudwins/zog"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	excel "github.com/negokaz/excel-mcp-server/internal/excel"
	imcp "github.com/negokaz/excel-mcp-server/internal/mcp"
)

type ReadSheetFormulaArguments struct {
	FileAbsolutePath  string   `zog:"fileAbsolutePath"`
	SheetName         string   `zog:"sheetName"`
	Range             string   `zog:"range"`
	KnownPagingRanges []string `zog:"knownPagingRanges"`
}

var readSheetFormulaArgumentsSchema = z.Struct(z.Schema{
	"fileAbsolutePath":  z.String().Test(AbsolutePathTest()).Required(),
	"sheetName":         z.String().Required(),
	"range":             z.String(),
	"knownPagingRanges": z.Slice(z.String()),
})

func AddReadSheetFormulaTool(server *server.MCPServer) {
	server.AddTool(mcp.NewTool("read_sheet_formula",
		mcp.WithDescription("Read formulas from Excel sheet with pagination."),
		mcp.WithString("fileAbsolutePath",
			mcp.Required(),
			mcp.Description("Absolute path to the Excel file"),
		),
		mcp.WithString("sheetName",
			mcp.Required(),
			mcp.Description("Sheet name in the Excel file"),
		),
		mcp.WithString("range",
			mcp.Description("Range of cells to read in the Excel sheet (e.g., \"A1:C10\"). [default: first paging range]"),
		),
		mcp.WithArray("knownPagingRanges",
			mcp.Description("List of already read paging ranges"),
			mcp.Items(map[string]any{
				"type": "string",
			}),
		),
	), handleReadSheetFormulaPaging)
}

func handleReadSheetFormulaPaging(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := ReadSheetFormulaArguments{}
	if issues := readSheetFormulaArgumentsSchema.Parse(request.Params.Arguments, &args); len(issues) != 0 {
		return imcp.NewToolResultZogIssueMap(issues), nil
	}
	return readSheetFormula(args.FileAbsolutePath, args.SheetName, args.Range, args.KnownPagingRanges)
}

func readSheetFormula(fileAbsolutePath string, sheetName string, valueRange string, knownPagingRanges []string) (*mcp.CallToolResult, error) {
	config, issues := LoadConfig()
	if issues != nil {
		return imcp.NewToolResultZogIssueMap(issues), nil
	}

	workbook, release, err := excel.OpenFile(fileAbsolutePath)
	if err != nil {
		return nil, err
	}
	defer release()

	worksheet, err := workbook.FindSheet(sheetName)
	if err != nil {
		return imcp.NewToolResultInvalidArgumentError(err.Error()), nil
	}

	// ページング戦略の初期化
	strategy, err := worksheet.GetPagingStrategy(config.EXCEL_MCP_PAGING_CELLS_LIMIT)
	if err != nil {
		return nil, err
	}
	pagingService := excel.NewPagingRangeService(strategy)

	// 利用可能な範囲を取得
	allRanges := pagingService.GetPagingRanges()
	if len(allRanges) == 0 {
		return imcp.NewToolResultInvalidArgumentError("no range available to read"), nil
	}

	// 現在の範囲を決定
	currentRange := valueRange
	if currentRange == "" && len(allRanges) > 0 {
		currentRange = allRanges[0]
	}

	// 残りの範囲を計算
	remainingRanges := pagingService.FilterRemainingPagingRanges(allRanges, append(knownPagingRanges, currentRange))

	// 範囲の検証
	if err := pagingService.ValidatePagingRange(currentRange); err != nil {
		return imcp.NewToolResultInvalidArgumentError(fmt.Sprintf("invalid range: %v", err)), nil
	}

	// 範囲を解析
	startCol, startRow, endCol, endRow, err := excel.ParseRange(currentRange)
	if err != nil {
		return nil, err
	}

	// HTMLテーブルの生成
	table, err := CreateHTMLTableOfFormula(worksheet, startCol, startRow, endCol, endRow)
	if err != nil {
		return nil, err
	}

	result := "<h2>Sheet Formulas</h2>\n"
	result += *table + "\n"
	result += "<h2>Metadata</h2>\n"
	result += "<ul>\n"
	result += fmt.Sprintf("<li>sheet name: %s</li>\n", html.EscapeString(sheetName))
	result += fmt.Sprintf("<li>read range: %s</li>\n", currentRange)
	result += "</ul>\n"
	result += "<h2>Notice</h2>\n"
	if len(remainingRanges) > 0 {
		result += "<p>This sheet has more some ranges.</p>\n"
		result += "<p>To read the next range, you should specify 'range' and 'knownPagingRanges' arguments as follows.</p>\n"
		result += fmt.Sprintf("<code>{ \"range\": \"%s\", \"knownPagingRanges\": [%s] }</code>\n", remainingRanges[0], "\""+strings.Join(append(knownPagingRanges, currentRange), "\", \"")+"\"")
	} else {
		result += "<p>All ranges have been read.</p>\n"
	}
	return mcp.NewToolResultText(result), nil
}

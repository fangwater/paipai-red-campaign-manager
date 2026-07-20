package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"paipai-red-campaign-manager/internal/guorai"

	"golang.org/x/term"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return errors.New("a command is required")
	}
	switch args[0] {
	case "login":
		return runLogin(ctx, args[1:])
	case "query":
		return runQuery(ctx, args[1:])
	case "export":
		return runExport(ctx, args[1:])
	case "sync":
		return runGuoraiSync(ctx, args[1:])
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	default:
		printUsage(os.Stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runLogin(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("login", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	username := flags.String("username", os.Getenv("GUORAI_USERNAME"), "Guorai account username")
	session := flags.String("session", defaultSessionPath(), "path to the persistent cookie session")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*username) == "" {
		return errors.New("--username or GUORAI_USERNAME is required")
	}
	password, err := readPassword()
	if err != nil {
		return err
	}
	client, err := guorai.NewClient(*session)
	if err != nil {
		return err
	}
	loginCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	if err := client.Login(loginCtx, *username, password); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "login succeeded; session saved to %s\n", *session)
	return nil
}

func runQuery(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("query", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	filter := registerFilterFlags(flags)
	session := flags.String("session", defaultSessionPath(), "path to the persistent cookie session")
	output := flags.String("output", "", "output file; defaults to stdout")
	format := flags.String("format", "", "json or csv; inferred from --output when omitted")
	pageSize := flags.Int("page-size", 200, "records requested per API page (max 500)")
	limit := flags.Int("limit", 0, "maximum records to return; 0 means all")
	if err := flags.Parse(args); err != nil {
		return err
	}
	filter.PageSize = *pageSize
	filter.Limit = *limit
	if filter.PageSize <= 0 || filter.PageSize > 500 {
		return errors.New("--page-size must be between 1 and 500")
	}
	if filter.Limit < 0 {
		return errors.New("--limit cannot be negative")
	}
	selectedFormat, err := outputFormat(*format, *output)
	if err != nil {
		return err
	}
	client, err := guorai.NewClient(*session)
	if err != nil {
		return err
	}
	result, err := client.QueryNotes(ctx, *filter)
	if err != nil {
		return err
	}
	data, err := encodeResult(result, selectedFormat)
	if err != nil {
		return err
	}
	if *output == "" {
		if _, err := os.Stdout.Write(data); err != nil {
			return err
		}
	} else if err := os.WriteFile(*output, data, 0o644); err != nil {
		return fmt.Errorf("write query output: %w", err)
	}
	fmt.Fprintf(os.Stderr, "queried %d of %d %s items for %s, %s through %s\n",
		len(result.Data), result.Total, result.Filter.BusinessType, result.Filter.BrandName,
		result.Filter.BeginDate, result.Filter.EndDate)
	return nil
}

func runExport(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	filter := registerFilterFlags(flags)
	session := flags.String("session", defaultSessionPath(), "path to the persistent cookie session")
	output := flags.String("output", "", "downloaded .xlsx path")
	noWait := flags.Bool("no-wait", false, "create the export task without waiting or downloading")
	timeout := flags.Duration("timeout", 15*time.Minute, "maximum time to wait for the export")
	poll := flags.Duration("poll-interval", 5*time.Second, "export status polling interval")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *timeout <= 0 || *poll <= 0 {
		return errors.New("--timeout and --poll-interval must be positive")
	}
	client, err := guorai.NewClient(*session)
	if err != nil {
		return err
	}
	result, err := client.StartNotesExport(ctx, guorai.ExportRequest{
		Filter: *filter, NoWait: *noWait, Timeout: *timeout, PollInterval: *poll, OutputPath: *output,
	})
	if err != nil {
		return err
	}
	if *noWait {
		fmt.Printf("export task created: %s\n", result.Task.TaskName)
		return nil
	}
	fmt.Printf("export downloaded: %s\n", result.OutputPath)
	return nil
}

func registerFilterFlags(flags *flag.FlagSet) *guorai.NotesFilter {
	var filter guorai.NotesFilter
	flags.StringVar(&filter.BusinessType, "type", guorai.BusinessTypeNote, "business type: note or plan")
	flags.StringVar(&filter.BeginDate, "from", "", "touch-time start date (YYYY-MM-DD); defaults to 7 days before cutoff")
	flags.StringVar(&filter.EndDate, "to", "", "touch-time end date (YYYY-MM-DD); defaults to statistics cutoff")
	flags.StringVar(&filter.BrandID, "brand-id", "", "XHS brand ID; defaults to the bound brand")
	flags.StringVar(&filter.MerchantID, "merchant-id", "", "merchant ID; omitted for all/default shop")
	flags.StringVar(&filter.SortField, "sort-field", "totalPayAmt", "metric field used for sorting")
	flags.StringVar(&filter.SortOrder, "sort-order", "DESC", "ASC or DESC")
	return &filter
}

func readPassword() (string, error) {
	if password := os.Getenv("GUORAI_PASSWORD"); password != "" {
		return password, nil
	}
	fmt.Fprint(os.Stderr, "Password: ")
	if term.IsTerminal(int(syscall.Stdin)) {
		password, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		if len(password) == 0 {
			return "", errors.New("password is required")
		}
		return string(password), nil
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read password: %w", err)
	}
	password := strings.TrimRight(line, "\r\n")
	if password == "" {
		return "", errors.New("password is required")
	}
	return password, nil
}

func outputFormat(raw, output string) (string, error) {
	format := strings.ToLower(strings.TrimSpace(raw))
	if format == "" {
		if strings.EqualFold(filepath.Ext(output), ".csv") {
			format = "csv"
		} else {
			format = "json"
		}
	}
	if format != "json" && format != "csv" {
		return "", errors.New("--format must be json or csv")
	}
	return format, nil
}

func encodeResult(result guorai.NotesResult, format string) ([]byte, error) {
	if format == "json" {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("encode JSON: %w", err)
		}
		return append(data, '\n'), nil
	}
	return encodeCSV(result.Data)
}

func encodeCSV(records []map[string]any) ([]byte, error) {
	preferred := []string{
		"planId", "planName", "planPublishTime", "planType",
		"noteId", "noteName", "noteAuthorName", "notePublishTime", "noteType", "spuName", "accountName",
		"totalPayUser", "totalPayOrder", "totalPayAmt", "consume", "noteAdCostVolume", "totalRoi",
	}
	keys := make(map[string]bool)
	for _, record := range records {
		for key := range record {
			keys[key] = true
		}
	}
	columns := make([]string, 0, len(keys))
	for _, key := range preferred {
		if keys[key] {
			columns = append(columns, key)
			delete(keys, key)
		}
	}
	remaining := make([]string, 0, len(keys))
	for key := range keys {
		remaining = append(remaining, key)
	}
	sort.Strings(remaining)
	columns = append(columns, remaining...)

	var buffer bytes.Buffer
	buffer.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buffer)
	if err := writer.Write(columns); err != nil {
		return nil, err
	}
	for _, record := range records {
		row := make([]string, len(columns))
		for index, key := range columns {
			row[index] = csvValue(record[key])
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func csvValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	}
}

func defaultSessionPath() string {
	if value := os.Getenv("GUORAI_SESSION_FILE"); value != "" {
		return value
	}
	return filepath.Join(".guorai", "session.json")
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage: guorai <command> [options]")
	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Commands:")
	fmt.Fprintln(writer, "  login   authenticate once and persist the Cookie session")
	fmt.Fprintln(writer, "  query   query followed notes or plans by touch time and output JSON/CSV")
	fmt.Fprintln(writer, "  export  create an Excel export task and download the result")
	fmt.Fprintln(writer, "  sync    refresh rolling snapshots and store them in PostgreSQL")
}

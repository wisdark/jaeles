package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jaeles-project/jaeles/core"
	"github.com/jaeles-project/jaeles/database"
	"github.com/jaeles-project/jaeles/libs"
	"github.com/jaeles-project/jaeles/utils"
	"github.com/jinzhu/gorm"
	"github.com/spf13/cobra"
	"github.com/thoas/go-funk"
)

var options = libs.Options{}

// DB database variables
var _ *gorm.DB

var RootCmd = &cobra.Command{
	Use:   "jaeles",
	Short: "Jaeles Scanner",
	Long:  libs.Banner(),
}

// Execute main function
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	// config options
	RootCmd.PersistentFlags().StringVar(&options.ConfigFile, "config", "", "config file (default is $HOME/.jaeles/config.yaml)")
	RootCmd.PersistentFlags().StringVar(&options.RootFolder, "rootDir", "~/.jaeles/", "root Project")
	RootCmd.PersistentFlags().StringVarP(&options.SignFolder, "signDir", "B", "~/.jaeles/base-signatures/", "Folder contain default signatures")
	RootCmd.PersistentFlags().StringVar(&options.ScanID, "scanID", "", "Scan ID")
	// http options
	RootCmd.PersistentFlags().StringVar(&options.Proxy, "proxy", "", "proxy")
	RootCmd.PersistentFlags().IntVar(&options.Timeout, "timeout", 20, "HTTP timeout")
	RootCmd.PersistentFlags().IntVar(&options.Retry, "retry", 0, "HTTP Retry")
	RootCmd.PersistentFlags().IntVar(&options.Delay, "delay", 0, "Delay time between requests")
	// output options
	RootCmd.PersistentFlags().StringVarP(&options.Output, "output", "o", "out", "Output folder name")
	RootCmd.PersistentFlags().StringVar(&options.PassiveOutput, "passiveOutput", "", "Passive output folder (default is passive-out)")
	RootCmd.PersistentFlags().StringVar(&options.PassiveSummary, "passiveSummary", "", "Passive Summary file")
	RootCmd.PersistentFlags().StringVarP(&options.SummaryOutput, "summaryOutput", "O", "", "Summary output file")
	RootCmd.PersistentFlags().StringVar(&options.SummaryVuln, "summaryVuln", "", "Summary output file")
	// report options
	RootCmd.PersistentFlags().StringVarP(&options.Report.ReportName, "report", "R", "", "Report name")
	RootCmd.PersistentFlags().StringVar(&options.Report.Title, "title", "", "Report title name")
	// core options
	RootCmd.PersistentFlags().BoolVarP(&options.EnablePassive, "passive", "G", false, "Turn on passive detections")
	RootCmd.PersistentFlags().IntVarP(&options.Level, "level", "L", 1, "Filter signature by level")
	RootCmd.PersistentFlags().StringVar(&options.SelectedPassive, "sp", "*", "Selector for passive detections")
	RootCmd.PersistentFlags().IntVarP(&options.Concurrency, "concurrency", "c", 20, "Set the concurrency level")
	RootCmd.PersistentFlags().StringVarP(&options.Selectors, "selectorFile", "S", "", "Signature selector from file")
	RootCmd.PersistentFlags().StringSliceVarP(&options.Signs, "signs", "s", []string{}, "Signature selector (Multiple -s flags are accepted)")
	RootCmd.PersistentFlags().StringSliceVarP(&options.Excludes, "exclude", "x", []string{}, "Exclude Signature selector (Multiple -x flags are accepted)")
	// custom params from cli
	RootCmd.PersistentFlags().StringSliceVarP(&options.Params, "params", "p", []string{}, "Custom params -p='foo=bar' (Multiple -p flags are accepted)")
	RootCmd.PersistentFlags().StringSliceVarP(&options.Headers, "headers", "H", []string{}, "Custom headers (e.g: -H 'Referer: {{.BaseURL}}') (Multiple -H flags are accepted)")
	// misc options
	RootCmd.PersistentFlags().StringVarP(&options.LogFile, "log", "l", "", "log file")
	RootCmd.PersistentFlags().StringVarP(&options.FoundCmd, "found", "f", "", "Run host OS command when vulnerable found")
	RootCmd.PersistentFlags().BoolVarP(&options.EnableFormatInput, "format-input", "J", false, "Enable special input format")
	RootCmd.PersistentFlags().BoolVar(&options.SaveRaw, "save-raw", false, "save raw request")
	RootCmd.PersistentFlags().BoolVar(&options.NoOutput, "no-output", false, "Do not store raw output")
	RootCmd.PersistentFlags().BoolVar(&options.NoBackGround, "no-background", false, "Do not run background task")
	RootCmd.PersistentFlags().BoolVar(&options.NoDB, "no-db", false, "Disable Database")
	RootCmd.PersistentFlags().BoolVar(&options.DisableParallel, "single", false, "Disable parallel mode (use this when you need logic in single signature")
	RootCmd.PersistentFlags().IntVar(&options.Refresh, "refresh", 10, "Refresh time for background task")
	RootCmd.PersistentFlags().StringVarP(&options.QuietFormat, "quietFormat", "Q", "{{.VulnURL}}", "Format for quiet output")
	RootCmd.PersistentFlags().BoolVarP(&options.Quiet, "quiet", "q", false, "Quiet Output")
	RootCmd.PersistentFlags().BoolVarP(&options.Verbose, "verbose", "v", false, "Verbose output")
	RootCmd.PersistentFlags().BoolVarP(&options.Version, "version", "V", false, "Print version of Jaeles")
	RootCmd.PersistentFlags().BoolVar(&options.Debug, "debug", false, "Debug")
	// some shortcut
	RootCmd.PersistentFlags().BoolVar(&options.BaseRoot, "ba", false, "Shortcut for -p 'BaseURL=[[.Raw]]' or -p 'root=[[.Raw]]'")
	RootCmd.PersistentFlags().BoolVar(&options.BurpProxy, "lc", false, "Shortcut for '--proxy http://127.0.0.1:8080'")
	RootCmd.PersistentFlags().BoolVar(&options.FullHelp, "hh", false, "Show full help message")
	RootCmd.SetHelpFunc(rootHelp)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// set some mics info
	fmt.Fprintf(os.Stderr, "Jaeles %v by %v\n", libs.VERSION, libs.AUTHOR)
	if options.Version {
		os.Exit(0)
	}
	if options.Debug {
		options.Verbose = true
	}
	if options.BurpProxy {
		options.Proxy = "http://127.0.0.1:8080"
	}
	if options.BaseRoot {
		options.Params = append(options.Params, "BaseURL=[[.Raw]]")
	}

	utils.InitLog(&options)
	core.InitConfig(&options)

	// Init DB
	var err error
	if !options.NoDB {
		_, err = database.InitDB(utils.NormalizePath(options.Server.DBPath))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't connect to DB at %v\n", options.Server.DBPath)
			os.Exit(-1)
		}
	}
}

// SelectSign select signature
func SelectSign() {
	var selectedSigns []string
	// read selector from File
	if options.Selectors != "" {
		options.Signs = append(options.Signs, utils.ReadingFileUnique(options.Selectors)...)
	}

	// default is all signature
	if len(options.Signs) == 0 {
		selectedSigns = core.SelectSign("**")
	}

	// search signature through Signatures table
	for _, signName := range options.Signs {
		selectedSigns = append(selectedSigns, core.SelectSign(signName)...)
		if !options.NoDB {
			Signs := database.SelectSign(signName)
			selectedSigns = append(selectedSigns, Signs...)
		}
	}

	// exclude some signature
	if len(options.Excludes) > 0 {
		for _, exclude := range options.Excludes {
			for index, sign := range selectedSigns {
				if strings.Contains(sign, exclude) {
					selectedSigns = append(selectedSigns[:index], selectedSigns[index+1:]...)
				}
				r, err := regexp.Compile(exclude)
				if err != nil {
					continue
				}
				if r.MatchString(sign) {
					selectedSigns = append(selectedSigns[:index], selectedSigns[index+1:]...)
				}
			}
		}
	}
	options.SelectedSigns = selectedSigns

	if len(selectedSigns) == 0 {
		fmt.Fprintf(os.Stderr, "[Error] No signature loaded\n")
		fmt.Fprintf(os.Stderr, "Use 'jaeles -h' for more information about a command.\n")
		os.Exit(1)
	}
	selectedSigns = funk.UniqString(selectedSigns)
	utils.InforF("Signatures Loaded: %v", len(selectedSigns))
	signInfo := fmt.Sprintf("Signature Loaded: ")
	for _, signName := range selectedSigns {
		signInfo += fmt.Sprintf("%v ", filepath.Base(signName))
	}
	utils.InforF(signInfo)

	// create new scan or group with old one
	var scanID string
	if options.ScanID == "" {
		scanID = database.NewScan(options, "scan", selectedSigns)
	} else {
		scanID = options.ScanID
	}
	utils.InforF("Start Scan with ID: %v", scanID)
	options.ScanID = scanID
}

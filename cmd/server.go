package cmd

import (
	"fmt"
	"log"
	"path"
	"strings"
	"sync"

	"github.com/jaeles-project/jaeles/database"
	"github.com/jaeles-project/jaeles/libs"
	"github.com/jaeles-project/jaeles/server"
	"github.com/thoas/go-funk"

	"github.com/jaeles-project/jaeles/core"
	"github.com/spf13/cobra"
)

var serverCmd *cobra.Command

func init() {
	// byeCmd represents the bye command
	var serverCmd = &cobra.Command{
		Use:   "server",
		Short: "Run server",
		Long:  `Start API Server`,
		RunE:  runServer,
	}
	serverCmd.Flags().StringP("sign", "s", "", "Provide custom header seperate by ','")
	// serverCmd.Flags().Int16P("level", "l", 1, "Provide custom header seperate by ';'")

	serverCmd.Flags().String("host", "127.0.0.1", "IP address to bind the server")
	serverCmd.Flags().String("port", "5000", "Port")
	RootCmd.AddCommand(serverCmd)

}

func runServer(cmd *cobra.Command, args []string) error {
	// DB connect
	dbPath := path.Join(options.RootFolder, "sqlite.db")
	db, err := database.InitDB(dbPath)
	if err != nil {
		panic("Error open databases")
	}
	defer db.Close()
	result := make(chan libs.Record)
	jobs := make(chan libs.Record, options.Concurrency)

	go func() {
		for {
			Signs := []string{}
			signName, _ := cmd.Flags().GetString("sign")
			// Get exactly signature
			if strings.HasSuffix(signName, ".yaml") {
				if core.FileExists(signName) {
					Signs = append(Signs, signName)
				}
			}
			if signName != "" {
				signs := core.SelectSign(signName)
				Signs = append(Signs, signs...)
			} else {
				signName = database.GetDefaultSign()
			}
			Signs = append(Signs, database.SelectSign(signName)...)
			libs.InforF("Signatures Loaded: %v", len(Signs))

			// create new scan or group with old one
			var scanID string
			if options.ScanID == "" {
				scanID = database.NewScan(options, "scan", Signs)
			} else {
				scanID = options.ScanID
			}
			record := <-result
			for _, signFile := range Signs {
				sign, err := core.ParseSign(signFile)
				if err != nil {
					log.Printf("Error loading sign: %v\n", signFile)
					continue
				}
				// parse sign as list or single
				if sign.Type == "list" || sign.Type == "single" || sign.Type == "" {
					url := record.OriginReq.URL
					sign.Target = core.ParseTarget(url)
					sign.Target = core.MoreVariables(sign.Target, options)
					for _, req := range sign.Requests {
						realReqs := core.ParseRequest(req, sign)
						if req.Repeat > 0 {
							for i := 0; i < req.Repeat; i++ {
								realReqs = append(realReqs, realReqs...)
							}
						}
						if len(realReqs) > 0 {
							for _, realReq := range realReqs {
								var realRec libs.Record
								realRec.Request = realReq
								realRec.Request.Target = sign.Target
								realRec.OriginReq = record.OriginReq
								realRec.Sign = sign
								realRec.ScanID = scanID

								jobs <- realRec
							}
						}
					}
				} else {
					// parse fuzz sign
					for _, req := range sign.Requests {
						core.ParseRequestFromServer(&record, req, sign)
						// send origin request
						originRes, err := core.JustSend(options, record.OriginReq)
						if err == nil {
							record.OriginRes = originRes
							if options.Verbose {
								fmt.Printf("[Sent-Origin] %v %v \n", record.OriginReq.Method, record.OriginReq.URL)
							}
						}

						Reqs := core.ParseFuzzRequest(record, sign)
						if record.Request.Repeat > 0 {
							for i := 0; i < record.Request.Repeat; i++ {
								Reqs = append(Reqs, Reqs...)
							}
						}

						if len(Reqs) > 0 {
							for _, Req := range Reqs {
								// if options.Debug {
								// 	fmt.Printf("Path: %v \n", Req.Path)
								// 	fmt.Printf("URL: %v \n", Req.URL)
								// 	fmt.Printf("Body: %v \n", Req.Body)
								// 	fmt.Printf("Headers: %v \n", Req.Headers)
								// 	fmt.Printf("--------------\n")
								// }
								Rec := record
								Rec.Request = Req
								Rec.Sign = sign
								Rec.ScanID = scanID

								jobs <- Rec
							}
						}
					}
				}

			}

		}
	}()

	/* Start sending request here */
	var wg sync.WaitGroup
	for i := 0; i < options.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for realRec := range jobs {
				// run middleware here
				req := realRec.Request
				if !funk.IsEmpty(req.Middlewares) {
					core.MiddleWare(&realRec, options)
				}
				// if middleware return a response skip sending the request
				if realRec.Response.StatusCode == 0 {
					res, err := core.JustSend(options, req)
					if err != nil {
						continue
					}
					realRec.Response = res
				}

				if options.Verbose {
					fmt.Printf("[Sent] %v %v %v %v\n", realRec.Request.Method, realRec.Request.URL, realRec.Response.Status, realRec.Response.ResponseTime)
				}
				core.Analyze(options, &realRec)
			}
		}()
	}

	host, _ := cmd.Flags().GetString("host")
	port, _ := cmd.Flags().GetString("port")
	bind := fmt.Sprintf("%v:%v", host, port)
	options.Bind = bind
	libs.InforF("Start API server at %v", fmt.Sprintf("http://%v/#/", bind))

	server.InitRouter(options, result)

	// wg.Wait()
	return nil
}

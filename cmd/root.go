package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"msdrp/src"
	"os"
	"time"
)

var (
	tcpPort int
	httpPort int
	verificationKey string
	serverAddr string
	prefix string
	fileServerDir string
	fileServerPort int
)

var rootCmd = &cobra.Command{Use: "msdrp",}

var serverCmd = &cobra.Command{
	Use: "server",
	Short: "server command",
	Run: func(cmd *cobra.Command, args []string) {
		checkParam("server")
		
		log.Println("Starting server，listening tcp port：", tcpPort, "， http port：", httpPort)
		svr := src.NewRPServer()
		svr.TcpPort = tcpPort
		svr.HttpPort = httpPort
		svr.VerificationKey = verificationKey
		if err := svr.Start(); err != nil {
			log.Fatalln(err)
		}
		defer svr.Close()
	},
}

var clientCmd = &cobra.Command{
	Use:"client",
	Short: "client command",
	Run: func(cmd *cobra.Command, args []string) {
		checkParam("client")
		cli := src.NewRPClient()
		cli.ServerAddr = fmt.Sprintf("%s:%d", serverAddr, tcpPort)
		cli.FileServerPort = fileServerPort
		cli.FileServerDir = fileServerDir
		cli.Prefix = prefix
		cli.VerificationKey = verificationKey
		go cli.StartFileServer()
		retry:
			log.Println("Starting client，connecting：", serverAddr, "， port：", tcpPort, "， file server is started：", fileServerPort)
			if err := cli.Start(); err != nil {
				log.Println(err)
				log.Println("Reconnect in 5 seconds...")
				time.Sleep(time.Second * 5)
				goto retry
			}
			defer cli.Close()
	},
}


func init(){

	serverCmd.PersistentFlags().IntVarP(&tcpPort,"tcp-port","t",0,"tcp port")
	serverCmd.PersistentFlags().IntVarP(&httpPort,"http-port","p",0,"http port")
	serverCmd.PersistentFlags().StringVarP(&verificationKey,"vkey","v","DKibZF5TXvic1g3kY","verification code")
	rootCmd.AddCommand(serverCmd)

	clientCmd.PersistentFlags().StringVarP(&serverAddr,"server-addr","s","127.0.0.1","server address")
	clientCmd.PersistentFlags().StringVarP(&prefix,"prefix","x","","prefix")
	pwd,_ := os.Getwd()
	clientCmd.PersistentFlags().StringVarP(&fileServerDir,"file-server-dir","f",pwd,"file server directory")
	clientCmd.PersistentFlags().IntVarP(&tcpPort,"tcp-port","t",0,"tcp port")
	clientCmd.PersistentFlags().IntVarP(&fileServerPort,"file-server-port","o",0,"file server port")
	clientCmd.PersistentFlags().StringVarP(&verificationKey,"vkey","v","DKibZF5TXvic1g3kY","verification code")
	rootCmd.AddCommand(clientCmd)

	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
}

func checkParam(side string){
	if tcpPort <= 0 || tcpPort >= 65536 {
		log.Fatalln("Invalid tcp port")
	}
	if verificationKey == "" {
		log.Fatalln("Verification key cannot be empty")
	}
	if side == "client"{
		if prefix == "" {
			log.Fatalln("prefix cannot be empty!")
		}
		if fileServerDir == ""{
			log.Fatalln("File server directory must be assigned!")
		}
	}else{
		if tcpPort == httpPort {
			log.Fatalln("tcp port and http port got to be different!")
		}
		if httpPort <= 0 || httpPort >= 65536 {
			log.Fatalln("Invalid http port")
		}
	}
	
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		rootCmd.Println(err)
		os.Exit(1)
	}
}

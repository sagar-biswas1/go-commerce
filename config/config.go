package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)


type Config struct{
	Version string
	ServiceName string
	HttpPort int
}

var config Config
func LoadConfig(){
	err:= godotenv.Load()
	if err!=nil{
		log.Fatal("Error while loading env")
		os.Exit(1)
	}
	version := os.Getenv("VERSION")
	if (version=="" ){
		log.Fatal("Version not defined in env")
		os.Exit(1)
	}

	serviceName:= os.Getenv("SERVICE_NAME")
	if (serviceName=="" ){
		log.Fatal("Service Name not defined in env")
		os.Exit(1)
	}

	httpPort:=os.Getenv("HTTP_PORT")

	if(httpPort==""){
		log.Fatal("Port not defined in env")
		os.Exit(1)
	}

	port ,err := strconv.ParseInt(httpPort,10,64)
	if(err !=nil){
		log.Fatal("Port must be an integer")
		os.Exit(1)
	}
	config =Config{
		Version: version,
		ServiceName: serviceName,
		HttpPort: int(port),
	}


}

func GetConfig()Config{
	return config
}
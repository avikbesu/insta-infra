package main

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/flags"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/docker/compose/v2/pkg/compose"
	"github.com/urfave/cli/v2"
)

//go:embed docker-compose.yaml
var baseDockerComposeYaml string

var nonServiceSuffix = []string{"-data", "-init", "-server"}

func main() {
	ctx := context.TODO()

	p := createDockerProject(ctx, baseDockerComposeYaml)

	srv, err := createDockerService()
	if err != nil {
		log.Fatalln("Failed to create docker service:", err)
	}

	app := &cli.App{
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			{
				Usage: "Run services",
				Action: func(cCtx *cli.Context) error {
					err := runServices(ctx, srv, p, cCtx.Args().Slice())
					if err != nil {
						log.Fatalln("Failed to run services:", err)
					}
					return nil
				},
			},
			{
				Name:    "run",
				Aliases: []string{"r"},
				Usage:   "Run services",
				Action: func(cCtx *cli.Context) error {
					fmt.Println("run task: ", cCtx.Args().First())
					return nil
				},
			},
			{
				Name:    "connect",
				Aliases: []string{"c"},
				Usage:   "Connect to a service",
				Action: func(cCtx *cli.Context) error {
					fmt.Println("connect task: ", cCtx.Args().First())
					return nil
				},
			},
			{
				Name:    "down",
				Aliases: []string{"d"},
				Usage:   "Bring all services down",
				Action: func(cCtx *cli.Context) error {
					err := downServices(ctx, srv, p, cCtx.Args().Slice())
					if err != nil {
						log.Fatalln("Failed to bring down services:", err)
					}
					return nil
				},
			},
			{
				Name:    "update",
				Aliases: []string{"u"},
				Usage:   "Update to the latest service versions",
				Action: func(cCtx *cli.Context) error {
					fmt.Println("update task: ", cCtx.Args().First())
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}

	connectToService(ctx, srv, p, "my-service", "echo hello world")
	connectToService(ctx, srv, p, "my-service", "cd / && ls")

	fmt.Println("Docker service down...")
	err = srv.Down(ctx, p.Name, api.DownOptions{})
	if err != nil {
		log.Fatalln("Failed to bring services down:", err)
	}
}

func runServices(ctx context.Context, srv api.Service, p *types.Project, services []string) error {
	log.Println("Attempting to being services up:", services)
	startOptions := api.StartOptions{Services: services}
	err := srv.Up(ctx, p, api.UpOptions{Start: startOptions})
	if err != nil {
		log.Fatalln("Failed to bring services up:", err)
	}
	return err
}

func downServices(ctx context.Context, srv api.Service, p *types.Project, services []string) error {
	log.Println("Attempting to being services down:", services)
	err := srv.Down(ctx, p.Name, api.DownOptions{Services: services})
	if err != nil {
		log.Fatalln("Failed to bring services down:", err)
	}
	return err
}

func getLatestDockerComposeFile(dockerComposeUrl string) {
	response, err := http.Get(dockerComposeUrl)
	if err != nil {
		log.Fatalln("Failed to get docker compose file from url:", dockerComposeUrl, err)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.Fatalln("Failed to close HTTP response body: ", err)
		}
	}(response.Body)

	body, err1 := io.ReadAll(response.Body)
	if err1 != nil {
		log.Fatalln("Failed to read in HTTP response body: ", err1)
	}

	err2 := os.WriteFile("docker-compose.yaml", body, 0644)
	if err2 != nil {
		log.Fatalln("Failed to write to docker-compose.yaml file: ", err2)
	}
}

func createDockerProject(ctx context.Context, data string) *types.Project {
	configDetails := types.ConfigDetails{
		WorkingDir: "/in-memory/", // Fake path, doesn't need to exist.
		ConfigFiles: []types.ConfigFile{
			{Filename: "docker-compose.yaml", Content: []byte(data)},
		},
		Environment: nil,
	}

	projectName := "testproject"

	p, err := loader.LoadWithContext(ctx, configDetails, func(options *loader.Options) {
		options.SetProjectName(projectName, true)
	})
	if err != nil {
		log.Fatalln("Failed to load docker:", err)
	}
	return p
}

func createDockerService() (api.Service, error) {
	var srv api.Service
	dockerCli, err := command.NewDockerCli()
	if err != nil {
		return srv, err
	}

	dockerContext := "default"

	myOpts := &flags.ClientOptions{Context: dockerContext, LogLevel: "error"}
	err = dockerCli.Initialize(myOpts)
	if err != nil {
		return srv, err
	}

	srv = compose.NewComposeService(dockerCli)

	return srv, nil
}

func connectToService(ctx context.Context, srv api.Service, p *types.Project, service string, cmd string) {
	result, err := srv.Exec(ctx, p.Name, api.RunOptions{
		Service:     service,
		Command:     []string{cmd},
		WorkingDir:  "/bin",
		Tty:         true,
		Environment: []string{},
	})
	if err != nil {
		log.Fatalln("Failed to connect to service:", service, ". Error:", err)
	}
	log.Println("Command result:", result, " and err:", err)
}

func printServices(cCtx *cli.Context, services types.Services) {
	// This will complete if no args are passed
	if cCtx.NArg() > 0 {
		return
	}
	for _, serviceConfig := range services {
		serviceName := serviceConfig.Name
		hasNonServiceSuffix := false
		for _, sfx := range nonServiceSuffix {
			if strings.HasSuffix(serviceName, sfx) {
				hasNonServiceSuffix = true
				break
			}
		}
		if hasNonServiceSuffix {
			fmt.Println(serviceConfig.Name)
		}
	}
}

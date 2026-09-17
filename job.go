// Copyright 2023 The Ryan SU Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/wf-platform/wf-account-job/internal/config"
	"github.com/wf-platform/wf-account-job/internal/envloader"
	"github.com/wf-platform/wf-account-job/internal/mqs/amq/task/dynamicperiodictask"
	"github.com/wf-platform/wf-account-job/internal/mqs/amq/task/mqtask"
	"github.com/wf-platform/wf-account-job/internal/mqs/amq/task/scheduletask"
	"github.com/wf-platform/wf-account-job/internal/server"
	"github.com/wf-platform/wf-account-job/internal/svc"
	"github.com/wf-platform/wf-account-job/types/job"
)

var configFile = flag.String("f", "", "the config file; defaults to the APP_ENV profile")

func main() {
	if err := envloader.Load("job"); err != nil {
		panic(err)
	}
	flag.Parse()

	var c config.Config
	if err := envloader.LoadConfig(*configFile, "job", &c); err != nil {
		panic(err)
	}
	ctx := svc.NewServiceContext(c)

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		job.RegisterJobServer(grpcServer, server.NewJobServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	serviceGroup := service.NewServiceGroup()
	defer func() {
		serviceGroup.Stop()
		logx.Close()
	}()

	serviceGroup.Add(s)

	serviceGroup.Add(mqtask.NewMQTask(ctx))
	if c.TaskConf.EnableDPTask {
		serviceGroup.Add(dynamicperiodictask.NewDPTask(ctx))
	}

	if c.TaskConf.EnableScheduledTask {
		serviceGroup.Add(scheduletask.NewSchedulerTask(ctx))
	}

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	serviceGroup.Start()
}

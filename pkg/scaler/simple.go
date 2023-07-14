/*
Copyright 2023 The Alibaba Cloud Serverless Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scaler

import (
	"container/list"
	"context"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"sync"
	"time"

	"github.com/AliyunContainerService/scaler/pkg/config"
	"github.com/AliyunContainerService/scaler/pkg/model"
	"github.com/AliyunContainerService/scaler/pkg/platform_client"
	pb "github.com/AliyunContainerService/scaler/proto"
	"github.com/google/uuid"
)

type Simple struct {
	config         *config.Config
	metaData       *model.Meta
	platformClient platform_client.Client
	mu             sync.Mutex
	wg             sync.WaitGroup
	instances      map[string]*model.Instance
	idleInstance   *list.List
	// Exec Times
	executionDuration float32
	startTime         map[string]*time.Time
	// AssignTime
	assignDuration float32
	lastAssignTime time.Time
}

func New(metaData *model.Meta, config *config.Config) Scaler {
	client, err := platform_client.New(config.ClientAddr)
	if err != nil {
		log.Fatalf("client init with error: %s", err.Error())
	}
	scheduler := &Simple{
		config:         config,
		metaData:       metaData,
		platformClient: client,
		mu:             sync.Mutex{},
		wg:             sync.WaitGroup{},
		instances:      make(map[string]*model.Instance),
		idleInstance:   list.New(),
		// Exec Times
		executionDuration: 0,
		startTime:         make(map[string]*time.Time),
		// AssignTime
		assignDuration: 0,
		lastAssignTime: time.Now(),
	}
	log.Printf("New         %s %d %d %s", metaData.GetKey(), metaData.GetMemoryInMb(), metaData.GetTimeoutInSecs(), metaData.GetRuntime())
	scheduler.wg.Add(1)
	go func() {
		defer scheduler.wg.Done()
		scheduler.gcLoop()
		log.Printf("gc loop for app: %s is stoped", metaData.Key)
	}()
	return scheduler
}

func (s *Simple) Assign(ctx context.Context, request *pb.AssignRequest) (*pb.AssignReply, error) {
	// START 调度 App Request
	log.Printf("Assign      %s  %s", request.MetaData.Key, request.RequestId[:8])

	start := time.Now()
	instanceId := uuid.New().String()
	defer func() {
		// START 运行  App Instance Request cost
		log.Printf("Run         %s  %s  %s  %d", request.MetaData.Key, instanceId[:8], request.RequestId, time.Since(start).Milliseconds())
	}()

	s.mu.Lock()
	// AssignTime
	assign_duration := float32(time.Since(s.lastAssignTime).Milliseconds())
	s.lastAssignTime = start
	if element := s.idleInstance.Front(); element != nil {
		s.assignDuration = s.assignDuration*0.2 + assign_duration*0.8 // AssignTime
		instance := element.Value.(*model.Instance)
		instance.Busy = true
		s.idleInstance.Remove(element)

		// Exec Times
		start_time := time.Now()
		s.startTime[instanceId] = &start_time

		s.mu.Unlock()
		instanceId = instance.Id
		return &pb.AssignReply{
			Status: pb.Status_Ok,
			Assigment: &pb.Assignment{
				RequestId:  request.RequestId,
				MetaKey:    instance.Meta.Key,
				InstanceId: instance.Id,
			},
			ErrorMessage: nil,
		}, nil
	}
	s.mu.Unlock()

	//CreateSlot
	resourceConfig := model.SlotResourceConfig{
		ResourceConfig: pb.ResourceConfig{
			MemoryInMegabytes: request.MetaData.MemoryInMb,
		},
	}
	log.Printf("CreateSlot  %s  %s  %d", request.MetaData.Key, request.RequestId[:8], time.Since(start).Milliseconds())
	slot, err := s.platformClient.CreateSlot(ctx, request.RequestId, &resourceConfig)
	if err != nil {
		errorMessage := fmt.Sprintf("create slot failed with: %s", err.Error())
		log.Printf("create slot failed with: %s", err.Error())
		return nil, status.Errorf(codes.Internal, errorMessage)
	}

	// // Try again
	// s.mu.Lock()
	// if element := s.idleInstance.Front(); element != nil {
	// 	instance := element.Value.(*model.Instance)
	// 	instance.Busy = true
	// 	s.idleInstance.Remove(element)
	// 	s.mu.Unlock()

	// 	// Create A new Idle Instance
	// 	defer func() {
	// 		s.deleteSlot(ctx, request.RequestId, slot.Id, instanceId, request.MetaData.Key, "Try Again")
	// 		// meta := &model.Meta{
	// 		// 	Meta: pb.Meta{
	// 		// 		Key:           request.MetaData.Key,
	// 		// 		Runtime:       request.MetaData.Runtime,
	// 		// 		TimeoutInSecs: request.MetaData.TimeoutInSecs,
	// 		// 	},
	// 		// }

	// 		// instanceId := uuid.New().String()
	// 		// new_instance, err := s.platformClient.Init(ctx, request.RequestId, instanceId, slot, meta)
	// 		// if err != nil {
	// 		// 	log.Printf("[Try Again]create instance failed with: %s", err.Error())
	// 		// }

	// 		// s.mu.Lock()
	// 		// new_instance.Busy = false
	// 		// s.idleInstance.PushFront(new_instance)
	// 		// s.instances[new_instance.Id] = new_instance
	// 		// s.mu.Unlock()
	// 	}()

	// 	instanceId = instance.Id
	// 	return &pb.AssignReply{
	// 		Status: pb.Status_Ok,
	// 		Assigment: &pb.Assignment{
	// 			RequestId:  request.RequestId,
	// 			MetaKey:    instance.Meta.Key,
	// 			InstanceId: instance.Id,
	// 		},
	// 		ErrorMessage: nil,
	// 	}, nil
	// }
	// s.mu.Unlock()

	// Init
	meta := &model.Meta{
		Meta: pb.Meta{
			Key:           request.MetaData.Key,
			Runtime:       request.MetaData.Runtime,
			TimeoutInSecs: request.MetaData.TimeoutInSecs,
		},
	}
	instance, err := s.platformClient.Init(ctx, request.RequestId, instanceId, slot, meta)
	if err != nil {
		errorMessage := fmt.Sprintf("create instance failed with: %s", err.Error())
		log.Printf("create instance failed with: %s", err.Error())
		return nil, status.Errorf(codes.Internal, errorMessage)
	}

	//add new instance
	s.mu.Lock()
	instance.Busy = true
	s.instances[instance.Id] = instance

	s.assignDuration = s.assignDuration*0.2 + assign_duration*0.8 // AssignTime

	// Exec Times
	start_time := time.Now()
	s.startTime[instanceId] = &start_time

	s.mu.Unlock()

	return &pb.AssignReply{
		Status: pb.Status_Ok,
		Assigment: &pb.Assignment{
			RequestId:  request.RequestId,
			MetaKey:    instance.Meta.Key,
			InstanceId: instance.Id,
		},
		ErrorMessage: nil,
	}, nil
}

func (s *Simple) Idle(ctx context.Context, request *pb.IdleRequest) (*pb.IdleReply, error) {
	if request.Assigment == nil {
		return nil, status.Errorf(codes.InvalidArgument, "assignment is nil")
	}
	reply := &pb.IdleReply{
		Status:       pb.Status_Ok,
		ErrorMessage: nil,
	}
	instanceId := request.Assigment.InstanceId

	// STOP 运行  App  Instance  Request
	log.Printf("Release     %s  %s  %s", request.Assigment.MetaKey, instanceId[:8], request.Assigment.RequestId[:8])

	needDestroy := false
	slotId := ""
	if request.Result != nil && request.Result.NeedDestroy != nil && *request.Result.NeedDestroy {
		needDestroy = true
	}

	defer func() {
		if needDestroy {
			s.deleteSlot(ctx, request.Assigment.RequestId, slotId, instanceId, request.Assigment.MetaKey, "bad instance")
		}
	}()

	s.mu.Lock()

	// Exec Times
	exe_time := time.Since(*s.startTime[instanceId]).Milliseconds()
	s.executionDuration = s.executionDuration*0.2 + float32(exe_time)*0.8
	// log.Printf("++++++++++ %s  %f ++++++++++++", s.metaData.Key, s.executionDuration)

	defer s.mu.Unlock()
	if instance := s.instances[instanceId]; instance != nil {
		slotId = instance.Slot.Id
		instance.LastIdleTime = time.Now()

		// Exec Times
		// if s.executionDuration >= 1200 && instance.InitDurationInMs <= 200 || s.assignDuration > 1000*60*5 {
		if s.executionDuration >= 1200 && instance.InitDurationInMs <= 200 && s.assignDuration > 1000*60 {
			needDestroy = true
			delete(s.instances, instanceId)
			log.Printf("request id %s, instance %s need be destroy", request.Assigment.RequestId, instanceId)
			return reply, nil
		}

		if needDestroy {
			delete(s.instances, instanceId)
			log.Printf("request id %s, instance %s need be destroy", request.Assigment.RequestId, instanceId)
			return reply, nil
		}

		if !instance.Busy {
			log.Printf("request id %s, instance %s already freed", request.Assigment.RequestId, instanceId)
			return reply, nil
		}
		instance.Busy = false
		s.idleInstance.PushFront(instance)
	} else {
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("request id %s, instance %s not found", request.Assigment.RequestId, instanceId))
	}
	return &pb.IdleReply{
		Status:       pb.Status_Ok,
		ErrorMessage: nil,
	}, nil
}

func (s *Simple) deleteSlot(ctx context.Context, requestId, slotId, instanceId, metaKey, reason string) {
	// STOP Slot
	log.Printf("DeleteSlot  %s  %s  %s  %s", metaKey, instanceId[:8], slotId[:8], requestId[:8])
	if err := s.platformClient.DestroySLot(ctx, requestId, slotId, reason); err != nil {
		log.Printf("DeleteSlotFailed  %s  delete Instance (Slot: %s) of app: %s failed with: %s", instanceId[:8], slotId, metaKey, err.Error())
	}
}

func (s *Simple) gcLoop() {
	log.Printf("gc loop for app: %s is started", s.metaData.Key)
	ticker := time.NewTicker(s.config.GcInterval)
	for range ticker.C {
		for {
			s.mu.Lock()
			if element := s.idleInstance.Back(); element != nil {
				instance := element.Value.(*model.Instance)
				idleDuration := time.Since(instance.LastIdleTime)
				if idleDuration > s.config.IdleDurationBeforeGC || s.idleInstance.Len() > 15 {
					//need GC
					s.idleInstance.Remove(element)
					delete(s.instances, instance.Id)
					s.mu.Unlock()
					// STOP  App  Instance  idleInstance.len  idleDuration
					log.Printf("GcLoop      %s  %s  %d  %fs", instance.Meta.GetKey(), instance.Id[:8], s.idleInstance.Len(), idleDuration.Seconds())
					go func() {
						reason := fmt.Sprintf("Idle duration: %fs, excceed configured duration: %fs", idleDuration.Seconds(), s.config.IdleDurationBeforeGC.Seconds())
						ctx := context.Background()
						ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
						defer cancel()
						s.deleteSlot(ctx, uuid.NewString(), instance.Slot.Id, instance.Id, instance.Meta.Key, reason)
					}()

					continue
				}
			}
			s.mu.Unlock()
			break
		}
	}
}

func (s *Simple) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Stats{
		TotalInstance:     len(s.instances),
		TotalIdleInstance: s.idleInstance.Len(),
	}
}

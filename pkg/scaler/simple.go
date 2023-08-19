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
	"log"
	"sync"
	// "sync/atomic" // Coldstart
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/AliyunContainerService/scaler/pkg/config"
	"github.com/AliyunContainerService/scaler/pkg/feature"
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
	// Lock
	waitingNum int
	// Feature
	feature *feature.AppFeature
	// ColdStart
	// coldStartNum int32
	// Exec Times
	// executionDurationInMs float32
	// startTime             map[string]*time.Time

	// Cycle
	// cycle_mu   sync.Mutex
	// cycle_time time.Time
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
		// Lock
		waitingNum: 0,
		// Feature
		feature: feature.AppFeatures[metaData.Key],
		// Cycle
		// cycle_mu:   sync.Mutex{},
		// cycle_time: time.Now().Add(-60 * time.Second),
		// Exec Times
		// executionDurationInMs: 0,
		// startTime:             make(map[string]*time.Time),
	}
	if scheduler.feature == nil {
		scheduler.feature = &feature.AppFeature{
			Key:              metaData.Key,
			ExecDurationInMs: 1000,
			InitDurationInMs: 1000,
			CycleInSec:       0,
			TermInSec:        0,
		}
	}
	log.Printf("New         %s %d %d %s", metaData.GetKey(), metaData.GetMemoryInMb(), metaData.GetTimeoutInSecs(), metaData.GetRuntime())
	scheduler.wg.Add(1)

	if num, ok := config.ColdStartNum[metaData.Key]; ok {
		go scheduler.PreAssignWithGroup(context.Background(), num)
		go func() {
			defer scheduler.wg.Done()
			scheduler.gcLoopForColdStartInstance()
			log.Printf("gc loop for app: %s is stoped", metaData.Key)
		}()
	} else {
		go func() {
			defer scheduler.wg.Done()
			scheduler.gcLoop()
			log.Printf("gc loop for app: %s is stoped", metaData.Key)
		}()
	}

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

	// Cycle
	// if app := feature.AppFeatures[request.MetaData.Key]; app != nil && app.CycleInSec > 0 {
	// 	go s.Cycle(context.Background(), app)
	// }

	if element := s.idleInstance.Front(); element != nil {
		instance := element.Value.(*model.Instance)
		instance.Busy = true
		s.idleInstance.Remove(element)

		// Exec Times
		// start_time := time.Now()
		// s.startTime[instanceId] = &start_time

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
	} else if
	// s.waitingNum < len(s.instances) &&
	// (s.feature.ExecDurationInMs < 100 ||
	// 	s.feature.ExecDurationInMs < s.feature.InitDurationInMs+100 && s.waitingNum < 3 ||
	// 	s.feature.ExecDurationInMs < s.feature.InitDurationInMs*3 && s.waitingNum < 1) {
	((s.feature.ExecDurationInMs < s.feature.InitDurationInMs+100) || s.feature.ExecDurationInMs < 400) &&
		s.waitingNum < s.config.MaxWaitingNum &&
		s.waitingNum < len(s.instances) {

		s.waitingNum += 1
		s.mu.Unlock()

		max_try := int((float32(s.feature.InitDurationInMs))/10) + 10
		for i := 0; i < max_try; i++ {
			time.Sleep(10 * time.Millisecond)
			s.mu.Lock()

			if element := s.idleInstance.Front(); element != nil {
				s.waitingNum -= 1
				instance := element.Value.(*model.Instance)
				instance.Busy = true
				s.idleInstance.Remove(element)
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
		}

		s.mu.Lock()
		s.waitingNum -= 1
	}

	s.mu.Unlock()

	// Coldstart
	// atomic.AddInt32(&s.coldStartNum, 1)

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
	s.feature.InitDurationInMs = instance.InitDurationInMs

	// Exec Times
	// start_time := time.Now()
	// s.startTime[instanceId] = &start_time

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
	// exe_time := time.Since(*s.startTime[instanceId]).Milliseconds()
	// s.executionDurationInMs = s.executionDurationInMs*0.3 + float32(exe_time)*0.7
	s.feature.ExecDurationInMs = int64((s.feature.ExecDurationInMs + int64(request.Result.DurationInMs)) / 2)

	defer s.mu.Unlock()
	if instance := s.instances[instanceId]; instance != nil {
		slotId = instance.Slot.Id
		instance.LastIdleTime = time.Now()

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

	interval := s.config.IdleDurationBeforeGC

	ticker := time.NewTicker(s.config.GcInterval)
	for range ticker.C {
		for {
			s.mu.Lock()
			if element := s.idleInstance.Back(); element != nil {
				instance := element.Value.(*model.Instance)
				idleDuration := time.Since(instance.LastIdleTime)
				if idleDuration > interval || s.idleInstance.Len() > 10 || (s.metaData.MemoryInMb >= 2048 && s.idleInstance.Len() > 1) {
					s.idleInstance.Remove(element)
					delete(s.instances, instance.Id)
					s.mu.Unlock()

					log.Printf("GcLoop      %s  %s  %d  %fs %d", instance.Meta.GetKey(), instance.Id[:8], s.idleInstance.Len(), idleDuration.Seconds(), interval)
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

func (s *Simple) gcLoopForColdStartInstance() {
	log.Printf("gc loop for app: %s is started", s.metaData.Key)

	interval := s.config.IdleDurationBeforeGC

	if sec, ok := s.config.IntervalInSec[s.metaData.Key]; ok {
		interval = time.Duration(sec) * time.Second
	}

	ticker := time.NewTicker(s.config.GcInterval)
	// i := 0
	for range ticker.C {
		// i = (i + 1) % 5
		// if i == 0 {
		// 	log.Printf("--------------- %s %v ----------------", s.metaData.Key, s.Stats())
		// }
		for {
			s.mu.Lock()
			if element := s.idleInstance.Back(); element != nil {
				instance := element.Value.(*model.Instance)
				idleDuration := time.Since(instance.LastIdleTime)
				if idleDuration > interval {
					s.idleInstance.Remove(element)
					delete(s.instances, instance.Id)
					s.mu.Unlock()

					log.Printf("GcLoop      %s  %s  %d  %fs %d", instance.Meta.GetKey(), instance.Id[:8], s.idleInstance.Len(), idleDuration.Seconds(), interval)
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
		// Coldstart
		// coldStartNum := atomic.LoadInt32(&s.coldStartNum)
		// if coldStartNum > 30 {
		// 	go s.PreAssign(context.Background())
		// }
		// atomic.StoreInt32(&s.coldStartNum, 0)
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

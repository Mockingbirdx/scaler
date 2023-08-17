package scaler

import (
	"context"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	// "sync"
	// "github.com/AliyunContainerService/scaler/pkg/feature"
	"time"

	"github.com/AliyunContainerService/scaler/pkg/model"
	pb "github.com/AliyunContainerService/scaler/proto"
	"github.com/google/uuid"
)

func (s *Simple) PreAssign(ctx context.Context) error {
	// START 调度 App Request
	log.Printf("Assign      %s [PreAssign]", s.metaData.GetKey())

	instanceId := uuid.New().String()

	//Create new Instance
	resourceConfig := model.SlotResourceConfig{
		ResourceConfig: pb.ResourceConfig{
			MemoryInMegabytes: s.metaData.GetMemoryInMb(),
		},
	}

	// START Slot App Instance Request cost
	log.Printf("CreateSlot  %s  %s", s.metaData.GetKey(), "[PreAssign]")
	slot, err := s.platformClient.CreateSlot(ctx, "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", &resourceConfig)
	if err != nil {
		errorMessage := fmt.Sprintf("create slot failed with: %s", err.Error())
		log.Printf("create slot failed with: %s", err.Error())
		return status.Errorf(codes.Internal, errorMessage)
	}

	meta := &model.Meta{
		Meta: pb.Meta{
			Key:           s.metaData.GetKey(),
			Runtime:       s.metaData.GetRuntime(),
			TimeoutInSecs: s.metaData.GetTimeoutInSecs(),
		},
	}

	instance, err := s.platformClient.Init(ctx, "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx", instanceId, slot, meta)
	if err != nil {
		errorMessage := fmt.Sprintf("create instance failed with: %s", err.Error())
		log.Printf("create instance failed with: %s", err.Error())
		return status.Errorf(codes.Internal, errorMessage)
	}

	//add new instance
	s.mu.Lock()
	instance.Busy = false
	s.instances[instance.Id] = instance
	s.idleInstance.PushFront(instance)
	s.mu.Unlock()

	return nil
}

func (s *Simple) PreAssignWithGroup(ctx context.Context, groupNum int) {
	if groupNum <= 0 {
		return
	}
	for i := 0; i < groupNum; i++ {
		go s.PreAssign(ctx)
	}
}

func (s *Simple) PreAssignWithInterval(ctx context.Context, sleep_duration time.Duration) error {
	// sleep
	time.Sleep(sleep_duration)

	return s.PreAssign(ctx)
}

// func (s *Simple) Cycle(ctx context.Context, app *feature.App) {
// 	now := time.Now()

// 	s.cycle_mu.Lock()
// 	if now.After(s.cycle_time.Add(10 * time.Second)) {
// 		s.cycle_time = now
// 		s.cycle_mu.Unlock()

// 		go s.PreAssign(ctx, time.Second*time.Duration(app.CycleInSec))
// 	} else {
// 		s.cycle_mu.Unlock()
// 		return
// 	}
// }

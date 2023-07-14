package scaler

import (
	"context"
	"fmt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
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

func (s *Simple) paLoop() {
	log.Printf("pre assign loop for app: %s is started", s.metaData.Key)
	ticker := time.NewTicker(s.config.PaInterval)
	for range ticker.C {
		s.mu.Lock()
		// there is active instance and no idle instance
		if len(s.instances) > s.idleInstance.Len() && s.idleInstance.Len() == 0 {
			s.PreAssign(context.Background())
		}
		s.mu.Unlock()
	}
}

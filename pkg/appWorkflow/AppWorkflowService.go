/*
 * Copyright (c) 2020 Devtron Labs
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package appWorkflow

import (
	"fmt"
	"github.com/devtron-labs/devtron/internal/sql/repository/appWorkflow"
	"github.com/devtron-labs/devtron/internal/sql/repository/pipelineConfig"
	"github.com/devtron-labs/devtron/internal/util"
	"github.com/devtron-labs/devtron/pkg/pipeline"
	"github.com/devtron-labs/devtron/pkg/sql"
	"github.com/go-pg/pg"
	"go.uber.org/zap"
	"time"
)

const (
	CI_PIPELINE_TYPE = "CI_PIPELINE"
	CD_PIPELINE_TYPE = "CD_PIPELINE"
)

type AppWorkflowService interface {
	CreateAppWorkflow(req AppWorkflowDto) (AppWorkflowDto, error)
	FindAppWorkflows(appId int) ([]AppWorkflowDto, error)
	FindAppWorkflowById(Id int, appId int) (AppWorkflowDto, error)
	DeleteAppWorkflow(appWorkflowId int, userId int32) error

	SaveAppWorkflowMapping(wf AppWorkflowMappingDto) (AppWorkflowMappingDto, error)
	FindAppWorkflowMapping(workflowId int) ([]AppWorkflowMappingDto, error)
	FindAppWorkflowMappingByComponent(id int, compType string) ([]*appWorkflow.AppWorkflowMapping, error)
	CheckCdPipelineByCiPipelineId(id int) bool
	FindAppWorkflowByName(name string, appId int) (AppWorkflowDto, error)

	FindAllWorkflowsComponentDetails(appId int) (*AllAppWorkflowComponentDetails, error)
}

type AppWorkflowServiceImpl struct {
	Logger                 *zap.SugaredLogger
	appWorkflowRepository  appWorkflow.AppWorkflowRepository
	dbPipelineOrchestrator pipeline.DbPipelineOrchestrator
	ciPipelineRepository   pipelineConfig.CiPipelineRepository
	pipelineRepository     pipelineConfig.PipelineRepository
}

type AppWorkflowDto struct {
	Id                    int                     `json:"id,omitempty"`
	Name                  string                  `json:"name"`
	AppId                 int                     `json:"appId"`
	AppWorkflowMappingDto []AppWorkflowMappingDto `json:"tree,omitempty"`
	UserId                int32                   `json:"-"`
}

type AppWorkflowMappingDto struct {
	Id            int    `json:"id,omitempty"`
	AppWorkflowId int    `json:"appWorkflowId"`
	Type          string `json:"type"`
	ComponentId   int    `json:"componentId"`
	ParentId      int    `json:"parentId"`
	ParentType    string `json:"parentType"`
	UserId        int32  `json:"-"`
}

type AllAppWorkflowComponentDetails struct {
	Workflows []*WorkflowComponentNamesDto `json:"workflows"`
}

type WorkflowComponentNamesDto struct {
	Id             int      `json:"id"`
	Name           string   `json:"name"`
	CiPipelineId   int      `json:"ciPipelineId"`
	CiPipelineName string   `json:"ciPipelineName"`
	CdPipelines    []string `json:"cdPipelines"`
}

func NewAppWorkflowServiceImpl(logger *zap.SugaredLogger, appWorkflowRepository appWorkflow.AppWorkflowRepository, dbPipelineOrchestrator pipeline.DbPipelineOrchestrator, ciPipelineRepository pipelineConfig.CiPipelineRepository, pipelineRepository pipelineConfig.PipelineRepository) *AppWorkflowServiceImpl {
	return &AppWorkflowServiceImpl{
		Logger:                 logger,
		appWorkflowRepository:  appWorkflowRepository,
		dbPipelineOrchestrator: dbPipelineOrchestrator,
		ciPipelineRepository:   ciPipelineRepository,
		pipelineRepository:     pipelineRepository,
	}
}

func (impl AppWorkflowServiceImpl) CreateAppWorkflow(req AppWorkflowDto) (AppWorkflowDto, error) {
	var wf *appWorkflow.AppWorkflow
	var savedAppWf *appWorkflow.AppWorkflow
	var err error

	if req.Id != 0 {
		wf = &appWorkflow.AppWorkflow{
			Id:   req.Id,
			Name: req.Name,
			AuditLog: sql.AuditLog{
				UpdatedOn: time.Now(),
				UpdatedBy: req.UserId,
			},
		}
		savedAppWf, err = impl.appWorkflowRepository.UpdateAppWorkflow(wf)
	} else {
		wf := &appWorkflow.AppWorkflow{
			Name:   req.Name,
			AppId:  req.AppId,
			Active: true,
			AuditLog: sql.AuditLog{
				CreatedOn: time.Now(),
				UpdatedOn: time.Now(),
				CreatedBy: req.UserId,
				UpdatedBy: req.UserId,
			},
		}
		savedAppWf, err = impl.appWorkflowRepository.SaveAppWorkflow(wf)
	}
	if err != nil {
		impl.Logger.Errorw("err", err)
		return req, err
	}
	req.Id = savedAppWf.Id
	return req, nil
}

func (impl AppWorkflowServiceImpl) FindAppWorkflows(appId int) ([]AppWorkflowDto, error) {
	appWorkflow, err := impl.appWorkflowRepository.FindByAppId(appId)
	if err != nil && err != pg.ErrNoRows {
		impl.Logger.Errorw("err", err)
		return nil, err
	}
	var workflows []AppWorkflowDto
	for _, w := range appWorkflow {
		workflow := AppWorkflowDto{
			Id:    w.Id,
			Name:  w.Name,
			AppId: w.AppId,
		}

		mapping, err := impl.FindAppWorkflowMapping(w.Id)
		if err != nil {
			return nil, err
		}
		workflow.AppWorkflowMappingDto = mapping
		workflows = append(workflows, workflow)
	}
	return workflows, err
}

func (impl AppWorkflowServiceImpl) FindAppWorkflowById(Id int, appId int) (AppWorkflowDto, error) {
	appWorkflow, err := impl.appWorkflowRepository.FindByIdAndAppId(Id, appId)
	if err != nil {
		impl.Logger.Errorw("err", err)
		return AppWorkflowDto{}, err
	}
	appWorkflowDto := &AppWorkflowDto{
		AppId: appWorkflow.AppId,
		Id:    appWorkflow.Id,
		Name:  appWorkflow.Name,
	}
	return *appWorkflowDto, err
}

func (impl AppWorkflowServiceImpl) DeleteAppWorkflow(appWorkflowId int, userId int32) error {
	impl.Logger.Debugw("Deleting app-workflow: ", "appWorkflowId", appWorkflowId)
	wf, err := impl.appWorkflowRepository.FindById(appWorkflowId)
	if err != nil {
		impl.Logger.Errorw("err", err)
		return err
	}

	mappingForCI, err := impl.appWorkflowRepository.FindWFCIMappingByWorkflowId(wf.Id)
	if err != nil {
		impl.Logger.Errorw("err", err)
		return err
	}
	if len(mappingForCI) > 0 {
		return &util.ApiError{
			InternalMessage:   "workflow has ci pipeline",
			UserDetailMessage: fmt.Sprintf("workflow has ci pipeline"),
			UserMessage:       fmt.Sprintf("workflow has ci pipeline")}
	}

	dbConnection := impl.pipelineRepository.GetConnection()
	tx, err := dbConnection.Begin()
	if err != nil {
		return err
	}
	// Rollback tx on error.
	defer tx.Rollback()

	// Deleting workflow
	err = impl.appWorkflowRepository.DeleteAppWorkflow(wf, tx)
	if err != nil {
		impl.Logger.Errorw("err", err)
		return err
	}
	// Delete app workflow mapping
	mapping, err := impl.appWorkflowRepository.FindWFAllMappingByWorkflowId(wf.Id)
	for _, item := range mapping {
		err := impl.appWorkflowRepository.DeleteAppWorkflowMapping(item, tx)
		if err != nil {
			impl.Logger.Errorw("error in deleting workflow mapping", "err", err)
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (impl AppWorkflowServiceImpl) SaveAppWorkflowMapping(req AppWorkflowMappingDto) (AppWorkflowMappingDto, error) {
	appWorkflow := &appWorkflow.AppWorkflowMapping{
		ParentId:      req.ParentId,
		AppWorkflowId: req.AppWorkflowId,
		ComponentId:   req.ComponentId,
		ParentType:    req.ParentType,
		Type:          req.Type,
		Active:        true,
		AuditLog: sql.AuditLog{
			CreatedOn: time.Now(),
			UpdatedOn: time.Now(),
			CreatedBy: req.UserId,
			UpdatedBy: req.UserId,
		},
	}
	dbConnection := impl.pipelineRepository.GetConnection()
	tx, err := dbConnection.Begin()
	if err != nil {
		return AppWorkflowMappingDto{}, err
	}
	// Rollback tx on error.
	defer tx.Rollback()

	appWorkflow, err = impl.appWorkflowRepository.SaveAppWorkflowMapping(appWorkflow, tx)
	if err != nil {
		impl.Logger.Errorw("err", err)
		return AppWorkflowMappingDto{}, err
	}
	req.Id = appWorkflow.Id

	err = tx.Commit()
	if err != nil {
		return AppWorkflowMappingDto{}, err
	}

	return AppWorkflowMappingDto{}, nil
}

func (impl AppWorkflowServiceImpl) FindAppWorkflowMapping(workflowId int) ([]AppWorkflowMappingDto, error) {
	appWorkflowMapping, err := impl.appWorkflowRepository.FindByWorkflowId(workflowId)
	if err != nil && err != pg.ErrNoRows {
		impl.Logger.Errorw("err", err)
		return nil, err
	}
	var workflows []AppWorkflowMappingDto
	for _, w := range appWorkflowMapping {
		workflow := AppWorkflowMappingDto{
			Id:            w.Id,
			ParentId:      w.ParentId,
			ComponentId:   w.ComponentId,
			Type:          w.Type,
			AppWorkflowId: w.AppWorkflowId,
			ParentType:    w.ParentType,
		}
		workflows = append(workflows, workflow)
	}
	return workflows, err
}

func (impl AppWorkflowServiceImpl) FindAppWorkflowMappingByComponent(id int, compType string) ([]*appWorkflow.AppWorkflowMapping, error) {
	appWorkflowMappings, err := impl.appWorkflowRepository.FindByComponent(id, compType)
	if err != nil && err != pg.ErrNoRows {
		impl.Logger.Errorw("err", err)
		return nil, err
	}
	return appWorkflowMappings, err
}

func (impl AppWorkflowServiceImpl) FindAppWorkflowByName(name string, appId int) (AppWorkflowDto, error) {
	appWorkflow, err := impl.appWorkflowRepository.FindByNameAndAppId(name, appId)
	if err != nil {
		impl.Logger.Errorw("err", err)
		return AppWorkflowDto{}, err
	}
	appWorkflowDto := &AppWorkflowDto{
		AppId: appWorkflow.AppId,
		Id:    appWorkflow.Id,
		Name:  appWorkflow.Name,
	}
	return *appWorkflowDto, err
}

func (impl AppWorkflowServiceImpl) CheckCdPipelineByCiPipelineId(id int) bool {
	appWorkflowMapping, err := impl.appWorkflowRepository.FindWFCDMappingByCIPipelineId(id)

	if err == nil && appWorkflowMapping != nil {
		return true
	}
	return false
}

func (impl AppWorkflowServiceImpl) FindAllWorkflowsComponentDetails(appId int) (*AllAppWorkflowComponentDetails, error) {
	//get all workflows
	appWorkflows, err := impl.appWorkflowRepository.FindByAppId(appId)
	if err != nil {
		impl.Logger.Errorw("error in getting app workflows by appId", "err", err, "appId", appId)
		return nil, err
	}
	appWorkflowMappings, err := impl.appWorkflowRepository.FindAllWFMappingsByAppId(appId)
	if err != nil {
		impl.Logger.Errorw("error in getting appWorkflowMappings by appId", "err", err, "appId", appId)
		return nil, err
	}
	var wfComponentDetails []*WorkflowComponentNamesDto
	wfIdAndComponentDtoIndexMap := make(map[int]int)
	for i, appWf := range appWorkflows {
		wfIdAndComponentDtoIndexMap[appWf.Id] = i
		wfComponentDetail := &WorkflowComponentNamesDto{
			Id:   appWf.Id,
			Name: appWf.Name,
		}
		wfComponentDetails = append(wfComponentDetails, wfComponentDetail)
	}

	//getting all ciPipelines by appId
	ciPipelines, err := impl.ciPipelineRepository.FindByAppId(appId)
	if err != nil {
		impl.Logger.Errorw("error in getting ciPipelines by appId", "err", err, "appId", appId)
		return nil, err
	}
	ciPipelineIdNameMap := make(map[int]string, len(ciPipelines))
	for _, ciPipeline := range ciPipelines {
		ciPipelineIdNameMap[ciPipeline.Id] = ciPipeline.Name
	}

	//getting all ciPipelines by appId
	cdPipelines, err := impl.pipelineRepository.FindActiveByAppId(appId)
	if err != nil {
		impl.Logger.Errorw("error in getting cdPipelines by appId", "err", err, "appId", appId)
		return nil, err
	}
	cdPipelineIdNameMap := make(map[int]string, len(cdPipelines))
	for _, cdPipeline := range cdPipelines {
		cdPipelineIdNameMap[cdPipeline.Id] = cdPipeline.Environment.Name
	}

	for _, appWfMapping := range appWorkflowMappings {
		if index, ok := wfIdAndComponentDtoIndexMap[appWfMapping.AppWorkflowId]; ok {
			if appWfMapping.Type == CI_PIPELINE_TYPE {
				wfComponentDetails[index].CiPipelineId = appWfMapping.ComponentId
				if name, ok1 := ciPipelineIdNameMap[appWfMapping.ComponentId]; ok1 {
					wfComponentDetails[index].CiPipelineName = name
				}
			} else if appWfMapping.Type == CD_PIPELINE_TYPE {
				if envName, ok1 := cdPipelineIdNameMap[appWfMapping.ComponentId]; ok1 {
					wfComponentDetails[index].CdPipelines = append(wfComponentDetails[index].CdPipelines, envName)
				}
			}
		}
	}
	resp := &AllAppWorkflowComponentDetails{
		Workflows: wfComponentDetails,
	}
	return resp, nil
}

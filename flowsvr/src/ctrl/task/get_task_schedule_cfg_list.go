
package task

import (
	"net/http"

	"github.com/czh0727/asy/flowsvr/src/constant"
	"github.com/czh0727/asy/flowsvr/src/db"
	"github.com/czh0727/asy/taskutils/rpc/model"
	"github.com/gin-gonic/gin"
	"github.com/niuniumart/gosdk/handler"
)

type GetTaskScheduleCfgListHandler struct {
	Req  model.GetTaskScheduleCfgListReq
	Resp model.GetTaskScheduleCfgListResp
}

func GetTaskScheduleCfgList(c *gin.Context) {
	var hd GetTaskScheduleCfgListHandler

	defer func() {
		hd.Resp.Msg = constant.GetErrMsg(
			hd.Resp.Code,
		)
		c.JSON(http.StatusOK, hd.Resp)
	}()

	_ = handler.Run(&hd)
}

func (p *GetTaskScheduleCfgListHandler) HandleInput() error {
	return nil
}

func (p *GetTaskScheduleCfgListHandler) HandleProcess() error {
	list, err := db.TaskTypeCfgNsp.GetTaskTypeCfgList(db.DB)
	if err != nil {
		p.Resp.Code = constant.ERR_GET_TASK_CFG_FROM_DB
		return err
	}

	p.Resp.ScheduleCfgList = make(
		[]*model.TaskScheduleCfg,
		0,
		len(list),
	)

	for _, item := range list {
		cfg := &model.TaskScheduleCfg{
			TaskType:          item.TaskType,
			ScheduleLimit:     item.ScheduleLimit,
			ScheduleInterval:  item.ScheduleInterval,
			MaxProcessingTime: item.MaxProcessingTime,
			MaxRetryNum:       item.MaxRetryNum,
			RetryInterval:     item.RetryInterval,
			MaxRetryInterval:  item.MaxRetryInterval,
			CreateTime:        item.CreateTime,
			ModifyTime:        item.ModifyTime,
		}

		p.Resp.ScheduleCfgList = append(
			p.Resp.ScheduleCfgList, cfg,
		)
	}

	return nil
}

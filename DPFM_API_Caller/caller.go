package dpfm_api_caller

import (
	"context"
	dpfm_api_input_reader "data-platform-api-article-releases-rmq-kube/DPFM_API_Input_Reader"
	dpfm_api_output_formatter "data-platform-api-article-releases-rmq-kube/DPFM_API_Output_Formatter"
	"data-platform-api-article-releases-rmq-kube/config"

	"github.com/latonaio/golang-logging-library-for-data-platform/logger"
	database "github.com/latonaio/golang-mysql-network-connector"
	rabbitmq "github.com/latonaio/rabbitmq-golang-client-for-data-platform"
	"golang.org/x/xerrors"
)

type DPFMAPICaller struct {
	ctx  context.Context
	conf *config.Conf
	rmq  *rabbitmq.RabbitmqClient
	db   *database.Mysql
}

func NewDPFMAPICaller(
	conf *config.Conf, rmq *rabbitmq.RabbitmqClient, db *database.Mysql,
) *DPFMAPICaller {
	return &DPFMAPICaller{
		ctx:  context.Background(),
		conf: conf,
		rmq:  rmq,
		db:   db,
	}
}

func (c *DPFMAPICaller) AsyncReleases(
	accepter []string,
	input *dpfm_api_input_reader.SDC,
	output *dpfm_api_output_formatter.SDC,
	log *logger.Logger,
) (interface{}, []error) {
	var response interface{}
	switch input.APIType {
	case "releases":
		response = c.releaseSqlProcess(input, output, accepter, log)
	default:
		log.Error("unknown api type %s", input.APIType)
	}
	return response, nil
}

func (c *DPFMAPICaller) releaseSqlProcess(
	input *dpfm_api_input_reader.SDC,
	output *dpfm_api_output_formatter.SDC,
	accepter []string,
	log *logger.Logger,
) *dpfm_api_output_formatter.Message {
	var headerData *dpfm_api_output_formatter.Header
	for _, a := range accepter {
		switch a {
		case "Header":
			h := c.headerRelease(input, output, log)
			headerData = h
			if h == nil  {
				continue
			}
		}
	}

	return &dpfm_api_output_formatter.Message{
		Header: headerData,
	}
}

func (c *DPFMAPICaller) headerRelease(
	input *dpfm_api_input_reader.SDC,
	output *dpfm_api_output_formatter.SDC,
	log *logger.Logger,
) (*dpfm_api_output_formatter.Header) {
	sessionID := input.RuntimeSessionID

	header := c.HeaderRead(input, log)
	if header == nil {
		return nil
	}
	header.IsReleaseled = input.Article.IsReleaseled
	res, err := c.rmq.SessionKeepRequest(nil, c.conf.RMQ.QueueToSQL()[0], map[string]interface{}{"message": header, "function": "ArticleHeader", "runtime_session_id": sessionID})
	if err != nil {
		err = xerrors.Errorf("rmq error: %w", err)
		log.Error("%+v", err)
		return nil
	}
	res.Success()
	if !checkResult(res) {
		output.SQLUpdateResult = getBoolPtr(false)
		output.SQLUpdateError = "Header Data cannot release"
		return nil
	}

// headerのリリースが取り消された時は子に影響を与えない
//	if !*header.IsReleaseled {
//		return header
//	}

	return header
}

func checkResult(msg rabbitmq.RabbitmqMessage) bool {
	data := msg.Data()
	d, ok := data["result"]
	if !ok {
		return false
	}
	result, ok := d.(string)
	if !ok {
		return false
	}
	return result == "success"
}

func getBoolPtr(b bool) *bool {
	return &b
}

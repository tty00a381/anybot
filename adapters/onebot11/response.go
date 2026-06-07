package onebot11

import (
	"encoding/json"
	"fmt"
)

// Response 表示 OneBot v11 动作响应，并保留原始 JSON。
type Response struct {
	Status  string          `json:"status"`
	RetCode int             `json:"retcode"`
	Data    json.RawMessage `json:"data,omitempty"`
	Message string          `json:"message,omitempty"`
	Wording string          `json:"wording,omitempty"`
	Echo    string          `json:"echo,omitempty"`

	Raw json.RawMessage `json:"-"`
}

// ActionError 描述一次被 OneBot v11 协议端拒绝或返回失败状态的动作调用。
type ActionError struct {
	Action  string
	Status  string
	RetCode int
	Message string
	Wording string
}

func (e *ActionError) Error() string {
	detail := e.Message
	if detail == "" {
		detail = e.Wording
	}
	if detail == "" {
		detail = "action failed"
	}
	return fmt.Sprintf("%s: status=%s retcode=%d: %s", e.Action, e.Status, e.RetCode, detail)
}

// UnmarshalJSON 兼容数字或字符串形式的 retcode。
func (r *Response) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Status  string          `json:"status"`
		RetCode flexibleInt64   `json:"retcode"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
		Wording string          `json:"wording"`
		Echo    string          `json:"echo"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	r.Status = decoded.Status
	r.RetCode = int(decoded.RetCode.int64())
	r.Data = decoded.Data
	r.Message = decoded.Message
	r.Wording = decoded.Wording
	r.Echo = decoded.Echo
	r.Raw = append(r.Raw[:0], data...)
	return nil
}

// OK 判断动作响应是否为成功状态。
func (r *Response) OK() bool {
	return r != nil && r.Status == "ok" && r.RetCode == 0
}

// Decode 将响应 data 字段解码到 out；空 data 会被视为成功空值。
func (r *Response) Decode(out any) error {
	if out == nil || r == nil || len(r.Data) == 0 || string(r.Data) == "null" {
		return nil
	}
	return json.Unmarshal(r.Data, out)
}

// sdk 包提供 AnyBot 插件开发 SDK。
//
// 普通插件作者应优先使用本包，而不是直接依赖 core。插件通常导出一个
// Plugin 定义，由运行框架在本地机器人目录中安装；Manifest.Name 只用于展示，
// PluginID 由宿主生成并作为配置、状态和私有数据目录的唯一身份。
//
// 最小插件形态：
//
//	var Plugin = sdk.Define(
//		sdk.Manifest{Name: "hello", Version: "1.0.0"},
//		Config{Command: "hello"},
//		func(ctx *sdk.Context, cfg Config) error {
//			ctx.Command(cfg.Command).Handle(func(c *sdk.EventContext) error {
//				_, err := c.ReplyText("hello")
//				return err
//			})
//			return nil
//		},
//	)
package sdk

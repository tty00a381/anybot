// onebot11 包提供 AnyBot 的 OneBot v11 适配器、消息段构造器和类型化动作客户端。
//
// 插件只需要回复消息时应优先使用 sdk 的协议无关消息能力；需要群管、文件、
// 好友列表等 OneBot v11 专属动作时，可以在 SDK 事件处理函数中调用
// ClientFrom(c) 或 EventFrom(c) 提取类型化对象。
package onebot11

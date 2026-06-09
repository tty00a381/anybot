// Package host 提供 AnyBot 面向最终用户的运行框架层。
//
// 本包负责读取机器人工作目录配置，管理基于 PluginID 的插件安装锁，
// 同步 plugins.d/<PluginID>.yaml，渲染生成宿主，并按框架拥有的持久化边界
// 装配 core.App。
//
// 普通插件模块应依赖 github.com/tty00a381/anybot/sdk。本包主要供 anybot CLI、
// 生成宿主和明确拥有机器人工作目录的高级嵌入代码使用。
package host

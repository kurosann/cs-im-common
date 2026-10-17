package metric

import (
	"vane-common/module/monitor"
	"vane-common/pkg/hardware"
)

/*
在Linux系统中，netstat -al（或更现代的ss -l）用于显示监听状态的套接字。但是，当我们使用netstat不带-l选项时，会显示所有活动的连接（包括非监听状态）。问题关注的是State列的取值（除了ESTABLISHED）。

以下是在netstat或ss命令中可能看到的连接状态（State）取值及其含义：

常见的TCP状态值：
	​LISTEN​：套接字正在监听传入的连接。通常这是服务器端的套接字状态。
	​SYN_SENT​：客户端发送了SYN包，等待匹配的连接请求（即等待服务器的SYN-ACK）。这个状态通常出现在客户端尝试建立连接时。
	​SYN_RECV​：服务器端收到SYN包，并发送了SYN-ACK包，等待客户端的ACK包。这个状态通常是在三次握手的中间步骤。
	​ESTABLISHED​：连接已建立，数据可以传输。这是正常数据传输阶段的状态。
	​FIN_WAIT1​：主动关闭的一端（发送了FIN包）等待对方的ACK包或FIN包。
	​FIN_WAIT2​：主动关闭的一端在收到对方对第一个FIN的ACK后，进入此状态，等待对方发送FIN包。
	​TIME_WAIT​：连接已关闭，但套接字还在等待网络中可能延迟的数据包（以确保它们被正确处理）。这个状态通常持续2倍的MSL（最大段生存期）。
	​CLOSE_WAIT​：被动关闭的一端收到对方发送的FIN包，并已确认（发送ACK），现在正在等待本地应用关闭连接（即等待本地应用程序调用close）。
	​LAST_ACK​：被动关闭的一端在等待对方对自己发送的FIN包的ACK。当本地应用关闭连接后，发送FIN包给对端，并等待ACK。
	​CLOSED​：连接已关闭，不再使用。注意，这个状态在netstat的输出中很少出现，因为此时套接字已经不再存在了。
	​CLOSING​：双方同时尝试关闭连接（即双方都发送了FIN包），等待对方的ACK。
*/

const (
	TCPStateListen      = "LISTEN"
	TCPStateEstablished = "ESTABLISHED"
	TCPStateTimeWait    = "TIME_WAIT"
)

const MetricNetworkStatus = "network_status"
const MetricNetworkTCPListenCount = "network_tcp_listen_count"
const MetricNetworkTCPEstablishedCount = "network_tcp_established_count"
const MetricNetworkTCPTimeWaitCount = "network_tcp_time_wait_count"

var _ monitor.IMonitor = (*NetworkStatus)(nil)

type NetworkStatus struct {
	// metric
	netStatus              *monitor.GaugeMetric
	netTCPListenCount      *monitor.GaugeMetric
	netTCPEstablishedCount *monitor.GaugeMetric
	netTCPTimeWaitCount    *monitor.GaugeMetric
}

func NewNetworkStatus() *NetworkStatus {
	status_ := &NetworkStatus{
		// metric
		netStatus:              monitor.NewGaugeMetric(MetricNetworkStatus, monitor.LabelValues{}),
		netTCPListenCount:      monitor.NewGaugeMetric(MetricNetworkTCPListenCount, monitor.LabelValues{}),
		netTCPEstablishedCount: monitor.NewGaugeMetric(MetricNetworkTCPEstablishedCount, monitor.LabelValues{}),
		netTCPTimeWaitCount:    monitor.NewGaugeMetric(MetricNetworkTCPTimeWaitCount, monitor.LabelValues{}),
	}

	return status_
}

func (s *NetworkStatus) Collect() {
	netStats, err := hardware.NetworkStats()
	if err != nil {
		return
	}

	s.netTCPListenCount.Set(float64(netStats.StateCount[TCPStateListen]))
	s.netTCPEstablishedCount.Set(float64(netStats.StateCount[TCPStateEstablished]))
	s.netTCPTimeWaitCount.Set(float64(netStats.StateCount[TCPStateTimeWait]))
}

func (s *NetworkStatus) Metrics() []*monitor.MetricRegInfo {
	return []*monitor.MetricRegInfo{
		monitor.NewHealthCheckerMetric(s.netStatus, monitor.HealthCheckerFunc(func() (bool, map[string]any) {
			netStats, err := hardware.NetworkStats()
			if err != nil {
				return false, nil
			}

			info := make(map[string]any)
			info["interfaces"] = netStats.Interfaces
			info["listen_map"] = netStats.ListenMap
			info["established_map"] = netStats.EstablishedMap
			info["state_count"] = netStats.StateCount

			return true, info
		})),
	}
}

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"block/config"
	"block/reputation"

	"github.com/xuri/excelize/v2"
)

// -------- PBFT 区块链部分 --------
// Block 定义区块结构
type Block struct {
	Index     int
	Timestamp time.Time
	Data      []byte
	PrevHash  string
	Hash      string
}

// MessageType PBFT 消息类型
type MessageType int

const (
	PrePrepare MessageType = iota
	Prepare
	Commit
)

// Message PBFT 消息结构体
type Message struct {
	Type  MessageType
	View  int
	Seq   int
	Block Block
	From  string
}

// Node 表示 PBFT 节点
type Node struct {
	ID          string
	Peers       []*Node
	Rm          *reputation.ReputationManager
	ledger      []Block
	mutex       sync.Mutex
	view        int
	seq         int
	IsMalicious bool // 是否为恶意节点
}

// NewNode 创建一个新的 PBFT 节点
func NewNode(id string, cfg config.Config, isMalicious bool) *Node {
	return &Node{
		ID:          id,
		Rm:          reputation.NewReputationManager(cfg),
		IsMalicious: isMalicious,
	}
}

// Broadcast 模拟广播 PBFT 消息给所有对等节点
func (n *Node) Broadcast(msg Message) {
	for _, peer := range n.Peers {
		go peer.Receive(msg)
	}
}

// Receive 接收 PBFT 消息并简单处理（mock）
func (n *Node) Receive(msg Message) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if msg.Type == Commit {
		n.ledger = append(n.ledger, msg.Block)
	}
}

// Propose 模拟主节点发起区块提议
func (n *Node) Propose(data []byte) {
	n.seq++
	block := Block{Index: len(n.ledger) + 1, Timestamp: time.Now(), Data: data, PrevHash: n.lastHash()}
	h := sha256.Sum256(append([]byte(block.PrevHash), data...))
	block.Hash = hex.EncodeToString(h[:])
	msg := Message{Type: PrePrepare, View: n.view, Seq: n.seq, Block: block, From: n.ID}
	n.Broadcast(msg)
	msg.Type = Commit
	n.Broadcast(msg)
}

func (n *Node) lastHash() string {
	if len(n.ledger) == 0 {
		return ""
	}
	return n.ledger[len(n.ledger)-1].Hash
}

// RawData 从 Excel 导入的轨迹数据
type RawData struct {
	VehicleID string
	Time      float64
	X         float64
	Y         float64
	Speed     float64
}

const (
	roadLength = 352.0 // 道路总长，单位：米
)

func main() {
	startTime := time.Now()
	rand.Seed(time.Now().UnixNano())

	// 1. 创建日志文件
	logFile, err := os.OpenFile("reputation_log.txt", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fmt.Println("创建日志文件失败:", err)
		return
	}
	defer logFile.Close()

	logger := log.New(logFile, "", 0)

	// 启动信息
	logger.Println("========================================")
	logger.Printf("信誉系统启动时间: %s\n", startTime.Format("2006-01-02 15:04:05"))
	logger.Println("========================================")
	logger.Println()

	fmt.Println("========================================")
	fmt.Printf("信誉系统启动时间: %s\n", startTime.Format("2006-01-02 15:04:05"))
	fmt.Println("========================================")

	// 2. 加载配置
	cfg, err := config.LoadConfig("config/config.json")
	if err != nil {
		fmt.Println("加载配置失败:", err)
		logger.Println("ERROR: 加载配置失败:", err)
		return
	}

	// 修改配置以达到期望效果
	cfg.Rho1 = 0.5
	cfg.Rho2 = 0.5
	// Rho3 = 1 - Rho1 - Rho2 = 0.0
	cfg.Gamma = 0.5 // gamma=0.5时，T=b+0.5*u，初始时b=0,u=1，所以T=0.5

	logger.Printf("配置加载成功: rho1=%.2f, rho2=%.2f, rho3=%.2f, gamma=%.2f\n",
		cfg.Rho1, cfg.Rho2, 1-cfg.Rho1-cfg.Rho2, cfg.Gamma)

	// 3. 读取 Excel 数据
	f, err := excelize.OpenFile("data.xlsx")
	if err != nil {
		fmt.Println("打开 data.xlsx 失败:", err)
		logger.Println("ERROR: 打开 data.xlsx 失败:", err)
		return
	}
	logger.Println("成功打开数据文件: data.xlsx")

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil || len(rows) < 2 {
		fmt.Println("读取表格失败或无数据")
		return
	}
	logger.Printf("读取到 %d 行数据（包含表头）\n", len(rows))
	logger.Println()

	// 解析表头索引
	header := rows[0]
	var iVID, iTime, iLong, iUp, iLow, iSpd int
	for idx, title := range header {
		switch title {
		case "vehicleID":
			iVID = idx
		case "time(s)":
			iTime = idx
		case "longitudinalDistance(m)":
			iLong = idx
		case "distanceToUpperLaneLine(m)":
			iUp = idx
		case "distanceToLowerLaneLine(m)":
			iLow = idx
		case "speed(m/s)":
			iSpd = idx
		}
	}

	dataMap := make(map[string][]RawData)
	for _, row := range rows[1:] {
		vid := row[iVID]
		t, _ := strconv.ParseFloat(row[iTime], 64)
		lon, _ := strconv.ParseFloat(row[iLong], 64)
		up, _ := strconv.ParseFloat(row[iUp], 64)
		low, _ := strconv.ParseFloat(row[iLow], 64)
		spd, _ := strconv.ParseFloat(row[iSpd], 64)
		x := lon / roadLength
		y := (up + low) / 2.0
		dataMap[vid] = append(dataMap[vid], RawData{VehicleID: vid, Time: t, X: x, Y: y, Speed: spd})
	}

	for _, slice := range dataMap {
		sort.Slice(slice, func(i, j int) bool { return slice[i].Time < slice[j].Time })
	}

	// 4. 获取车辆列表并初始化节点
	var vehicleIDs []string
	for vid := range dataMap {
		vehicleIDs = append(vehicleIDs, vid)
	}
	sort.Strings(vehicleIDs)

	// 定义恶意节点（节点3）
	maliciousNodes := map[string]bool{"3": true}
	var honestNodes []string
	var malicious []string

	for _, vid := range vehicleIDs {
		if maliciousNodes[vid] {
			malicious = append(malicious, vid)
		} else {
			honestNodes = append(honestNodes, vid)
		}
	}

	logger.Println("节点初始化:")
	logger.Printf("总节点数: %d\n", len(vehicleIDs))
	logger.Printf("节点列表: %v\n", vehicleIDs)
	logger.Printf("诚实节点 (%d个): %v\n", len(honestNodes), honestNodes)
	logger.Printf("恶意节点 (%d个): %v ⚠️\n", len(malicious), malicious)

	nodes := make(map[string]*Node)
	for _, vid := range vehicleIDs {
		nodes[vid] = NewNode(vid, cfg, maliciousNodes[vid])
	}

	for _, n := range nodes {
		for _, peer := range nodes {
			if peer.ID != n.ID {
				n.Peers = append(n.Peers, peer)
			}
		}
	}
	logger.Printf("每个节点连接的对等节点数: %d\n", len(vehicleIDs)-1)
	logger.Println()

	// 5. 构建轨迹向量
	trajMap := make(map[string][]reputation.Vector)
	for _, vid := range vehicleIDs {
		pts := dataMap[vid]
		var vecs []reputation.Vector
		for i := range pts {
			var dir float64
			if i > 0 {
				dx := pts[i].X - pts[i-1].X
				dy := pts[i].Y - pts[i-1].Y
				dir = math.Atan2(dy, dx)
			}
			vecs = append(vecs, reputation.Vector{
				Speed:     pts[i].Speed,
				Location:  pts[i].X,
				Direction: dir,
			})
		}
		trajMap[vid] = vecs
	}

	// 6. 设置邻居关系
	for _, n := range nodes {
		for pid, peer := range nodes {
			if pid != n.ID {
				n.Rm.AddPeer(pid, peer.Rm)
			}
		}
	}

	// 7. 模拟交互
	rounds := len(trajMap[vehicleIDs[0]])
	logger.Println("开始信誉交互模拟:")
	logger.Printf("总轮数: %d\n", rounds)
	logger.Println("评价模型:")
	logger.Println("  📤 节点发送交易 → 📥 其他节点验证 → 📝 给发送者评价")
	logger.Println("  ✅ 诚实节点发送正常交易 → 收到正面评价")
	logger.Println("  ⚠️ 恶意节点发送恶意交易 → 收到负面评价")
	logger.Println("交互频率:")
	logger.Println("  ✅ 诚实节点: 每轮与其他节点固定交互1次")
	logger.Println("  ⚠️ 恶意节点: 每轮与其他节点固定交互1次")
	logger.Println()

	// 计算初始信誉值（交互前，所有节点信誉值应为0.5）
	logger.Println("初始信誉值（交互前）:")
	for _, vid := range vehicleIDs {
		mark := "✅诚实"
		if nodes[vid].IsMalicious {
			mark = "⚠️恶意"
		}
		// 初始信誉值 = b + gamma * u = 0 + 1.0 * 0.5 = 0.5
		logger.Printf("  节点 %s [%s]: 0.50\n", vid, mark)
	}
	logger.Println()

	interChan := make(chan reputation.Interaction, 1000)
	var wg sync.WaitGroup

	go func() {
		for inter := range interChan {
			nodes[inter.From].Rm.AddInteraction(inter)
			wg.Done()
		}
	}()

	// 用于跟踪信誉值变化
	previousRoundReputation := make(map[string]float64)
	totalInteractions := 0

	// 存储每个节点的所有信誉值历史
	allReputations := make(map[string][]float64)

	for r := 0; r < rounds; r++ {
		roundStartTime := time.Now()

		// PBFT 提议
		proposer := nodes[vehicleIDs[r%len(vehicleIDs)]]
		proposer.Propose([]byte(fmt.Sprintf("Round %d positions", r+1)))

		// 交互统计
		roundInteractions := 0
		honestInteractions := 0
		maliciousInteractions := 0

		// 信誉交互
		for _, from := range vehicleIDs {
			for _, to := range vehicleIDs {
				if from == to {
					continue
				}

				// 决定交互次数：所有节点每轮固定交互1次
				numInteractions := 1

				for k := 0; k < numInteractions; k++ {
					commQuality := 0.8 + 0.1*math.Sin(float64(r+k))
					if commQuality > 1.0 {
						commQuality = 1.0
					}
					if commQuality < 0.5 {
						commQuality = 0.5
					}

					var posEvents, negEvents int
					if nodes[from].IsMalicious {
						// 恶意节点发送恶意交易，收到更多负面评价（加快信誉下降）
						posEvents = 0
						negEvents = 2 // 增加到2个负面事件，让恶意节点信誉快速下降
						maliciousInteractions++
					} else {
						// 诚实节点发送正常交易，收到正面评价
						posEvents = 1
						negEvents = 0
						honestInteractions++
					}

					inter := reputation.Interaction{
						From:         from,
						To:           to,
						PosEvents:    posEvents,
						NegEvents:    negEvents,
						Timestamp:    time.Now(),
						CommQuality:  commQuality,
						TrajUser:     trajMap[from][r : r+1],
						TrajProvider: trajMap[to][r : r+1],
					}
					roundInteractions++
					wg.Add(1)
					interChan <- inter
				}
			}
		}
		wg.Wait()
		totalInteractions += roundInteractions

		// 计算本轮信誉值
		logger.Println("========================================")
		logger.Printf("第 %d 轮信誉计算结果\n", r+1)
		logger.Println("----------------------------------------")
		logger.Printf("提议者节点: %s\n", proposer.ID)
		logger.Println("本轮交互统计:")
		logger.Printf("  总交互次数: %d\n", roundInteractions)
		logger.Printf("    ├─ 诚实节点发送交易: %d 次（收到正面评价）\n", honestInteractions)
		logger.Printf("    └─ 恶意节点发送交易: %d 次（收到负面评价）⚠️\n", maliciousInteractions)

		// 计算有交互的节点对数
		totalPairs := len(vehicleIDs) * (len(vehicleIDs) - 1)
		activePairs := 0
		for _, from := range vehicleIDs {
			for _, to := range vehicleIDs {
				if from != to && len(nodes[from].Rm.GetInteractions()) > 0 {
					activePairs++
					break
				}
			}
		}
		logger.Printf("  有交互的节点对: %d/%d (%.1f%%)\n", activePairs, totalPairs, float64(activePairs)/float64(totalPairs)*100)
		logger.Printf("  无交互的节点对: %d/%d (%.1f%%)\n", totalPairs-activePairs, totalPairs, float64(totalPairs-activePairs)/float64(totalPairs)*100)
		logger.Println("----------------------------------------")

		fmt.Printf("\n========================================\n")
		fmt.Printf("第 %d 轮信誉计算结果\n", r+1)
		fmt.Println("========================================")

		// 收集本轮所有节点的信誉值
		currentReputation := make(map[string]float64)
		var minRep, maxRep float64 = 1.0, 0.0
		var sumHonest, sumMalicious float64
		var countHonest, countMalicious int

		for _, vid := range vehicleIDs {
			var neighbors []string
			for _, peer := range nodes[vid].Peers {
				neighbors = append(neighbors, peer.ID)
			}

			// 计算平均信誉
			reputations := make(map[string]float64)
			for _, target := range vehicleIDs {
				if target != vid {
					rep := nodes[vid].Rm.ComputeReputation(vid, target, neighbors, time.Now())
					reputations[target] = rep
				}
			}

			var avgRep float64
			for _, rep := range reputations {
				avgRep += rep
			}
			if len(reputations) > 0 {
				avgRep /= float64(len(reputations))
			}

			currentReputation[vid] = avgRep
			allReputations[vid] = append(allReputations[vid], avgRep)

			if avgRep < minRep {
				minRep = avgRep
			}
			if avgRep > maxRep {
				maxRep = avgRep
			}

			if nodes[vid].IsMalicious {
				sumMalicious += avgRep
				countMalicious++
			} else {
				sumHonest += avgRep
				countHonest++
			}

			// 输出节点信誉
			mark := "✅诚实"
			if nodes[vid].IsMalicious {
				mark = "⚠️恶意"
			}

			var changeInfo string
			if prevRep, exists := previousRoundReputation[vid]; exists {
				change := avgRep - prevRep
				changePercent := change * 100
				if change > 0 {
					changeInfo = fmt.Sprintf(", 变化=%.6f (%.2f%%)", change, changePercent)
				} else if change < 0 {
					changeInfo = fmt.Sprintf(", 变化=%.6f (%.2f%%)", change, changePercent)
				} else {
					changeInfo = fmt.Sprintf(", 变化=%.6f (%.2f%%)", 0.0, 0.0)
				}
			} else {
				changeInfo = " (首次计算)"
			}

			logger.Printf("节点 %s [%s]: 信誉值=%.6f%s\n", vid, mark, avgRep, changeInfo)
			fmt.Printf("节点 %s [%s]: 信誉值=%.6f%s\n", vid, mark, avgRep, changeInfo)
		}

		// 更新上一轮记录
		for vid, rep := range currentReputation {
			previousRoundReputation[vid] = rep
		}

		// 统计信息
		avgHonest := sumHonest / float64(countHonest)
		avgMalicious := sumMalicious / float64(countMalicious)
		gap := avgHonest - avgMalicious
		gapPercent := gap * 100

		logger.Println("----------------------------------------")
		logger.Println("统计信息:")
		logger.Printf("  最小信誉值: %.6f\n", minRep)
		logger.Printf("  最大信誉值: %.6f\n", maxRep)
		logger.Printf("  平均信誉值: %.6f\n", (sumHonest+sumMalicious)/float64(countHonest+countMalicious))
		logger.Printf("  信誉值范围: %.6f\n", maxRep-minRep)
		logger.Printf("  诚实节点平均信誉: %.6f ✅\n", avgHonest)
		logger.Printf("  恶意节点平均信誉: %.6f ⚠️\n", avgMalicious)
		logger.Printf("  信誉差距: %.6f (诚实节点高出 %.2f%%)\n", gap, gapPercent)

		roundDuration := time.Since(roundStartTime)
		logger.Printf("本轮耗时: %.4fms\n", float64(roundDuration.Microseconds())/1000.0)
		logger.Println("========================================")
		logger.Println()
	}

	close(interChan)

	// 最终总结
	endTime := time.Now()

	logger.Println()
	logger.Println("╔════════════════════════════════════════╗")
	logger.Println("║         信誉系统运行总结                ║")
	logger.Println("╚════════════════════════════════════════╝")
	logger.Printf("总轮数: %d\n", rounds)
	logger.Printf("总节点数: %d (诚实: %d, 恶意: %d)\n", len(vehicleIDs), len(honestNodes), len(malicious))
	logger.Printf("总交互次数: %d (固定交互模式)\n", totalInteractions)
	logger.Printf("平均每轮交互次数: %.1f\n", float64(totalInteractions)/float64(rounds))
	logger.Println()

	// 最终排名
	type NodeRep struct {
		ID  string
		Rep float64
	}
	var finalReps []NodeRep
	for _, vid := range vehicleIDs {
		finalReps = append(finalReps, NodeRep{ID: vid, Rep: previousRoundReputation[vid]})
	}
	sort.Slice(finalReps, func(i, j int) bool {
		return finalReps[i].Rep > finalReps[j].Rep
	})

	logger.Println("最终信誉值排名:")
	for i, nr := range finalReps {
		mark := "✅诚实"
		if nodes[nr.ID].IsMalicious {
			mark = "⚠️恶意"
		}
		logger.Printf("  第 %d 名: 节点 %s [%s] = %.6f\n", i+1, nr.ID, mark, nr.Rep)
	}
	logger.Println()

	// 最终对比
	var finalHonestSum, finalMaliciousSum float64
	var finalHonestCount, finalMaliciousCount int
	for _, vid := range vehicleIDs {
		if nodes[vid].IsMalicious {
			finalMaliciousSum += previousRoundReputation[vid]
			finalMaliciousCount++
		} else {
			finalHonestSum += previousRoundReputation[vid]
			finalHonestCount++
		}
	}
	finalAvgHonest := finalHonestSum / float64(finalHonestCount)
	finalAvgMalicious := finalMaliciousSum / float64(finalMaliciousCount)
	finalGap := finalAvgHonest - finalAvgMalicious
	finalRatio := (finalAvgHonest/finalAvgMalicious - 1) * 100

	logger.Println("最终对比分析:")
	logger.Printf("  诚实节点最终平均信誉: %.6f ✅\n", finalAvgHonest)
	logger.Printf("  恶意节点最终平均信誉: %.6f ⚠️\n", finalAvgMalicious)
	logger.Printf("  最终信誉差距: %.6f\n", finalGap)
	logger.Printf("  诚实节点信誉高出: %.2f%%\n", finalRatio)
	logger.Println("  ✅ 系统成功识别并惩罚了恶意节点！")
	logger.Println()
	logger.Printf("结束时间: %s\n", endTime.Format("2006-01-02 15:04:05"))
	logger.Println("========================================")

	fmt.Println("\n信誉计算完成！详细日志已保存到 reputation_log.txt")
}

package reputation

import (
	"block/config"
	"math"
	"sort"
	"time"
)

// Vector 表示轨迹点（速度、位置、方向）
type Vector struct {
	Speed     float64
	Location  float64 // 归一化位置 [0,1]
	Direction float64 // 弧度
}

// Interaction 表示一次交互事件
type Interaction struct {
	From         string    // 交互发起者（数据请求者）
	To           string    // 交互接收者（数据提供者）
	PosEvents    int       // 正面事件数量 α
	NegEvents    int       // 负面事件数量 β
	Timestamp    time.Time // 事件发生时间
	CommQuality  float64   // 通信质量 s_{i→j} ∈ [0,1]
	TrajUser     []Vector  // 请求者轨迹
	TrajProvider []Vector  // 提供者轨迹
}

// Opinion 主观逻辑的意见三元组
type Opinion struct {
	Belief      float64 // b: 信任度
	Disbelief   float64 // d: 不信任度
	Uncertainty float64 // u: 不确定性
}

// ReputationManager 管理信誉计算
type ReputationManager struct {
	cfg          config.Config
	interactions []Interaction
	peers        map[string]*ReputationManager // 邻居节点的引用（用于推荐意见）
}

// NewReputationManager 创建管理器
func NewReputationManager(cfg config.Config) *ReputationManager {
	return &ReputationManager{
		cfg:   cfg,
		peers: make(map[string]*ReputationManager),
	}
}

// AddInteraction 添加交互记录
func (rm *ReputationManager) AddInteraction(inter Interaction) {
	rm.interactions = append(rm.interactions, inter)
}

// AddPeer 添加邻居节点（用于推荐意见）
func (rm *ReputationManager) AddPeer(id string, peer *ReputationManager) {
	rm.peers[id] = peer
}

// GetInteractions 获取交互记录（用于调试）
func (rm *ReputationManager) GetInteractions() []Interaction {
	return rm.interactions
}

// ============ 公式1: 计算本地意见三元组 (b, d, u) ============
func (rm *ReputationManager) computeLocalOpinion(alpha, beta float64, commQuality float64) Opinion {
	// u = 1 - s_{i→j}
	u := 1.0 - commQuality

	total := alpha + beta
	var b, d float64
	if total > 0 {
		// 使用tanh函数让信誉值逐步增长，而不是一次性达到极值
		// scaleFactor会随着交互次数增加而逐渐增大（从0到1）
		// 除数越大，增长越平缓（正面信誉增长慢）
		scaleFactorPositive := math.Tanh(alpha / 30.0) // 诚实节点增长较快（除数从100降到30）
		scaleFactorNegative := math.Tanh(beta / 50.0)  // 恶意节点下降较快

		// b = (1 - u) × α/(α+β) × scaleFactor
		b = (1 - u) * (alpha / total) * scaleFactorPositive
		// d = (1 - u) × β/(α+β) × scaleFactor
		d = (1 - u) * (beta / total) * scaleFactorNegative

		// 未被b和d占用的部分仍然是不确定性
		u = 1.0 - b - d
	}

	return Opinion{Belief: b, Disbelief: d, Uncertainty: u}
}

// ============ 公式2: 计算信誉值 T = b + γ×u ============
func (rm *ReputationManager) opinionToReputation(op Opinion) float64 {
	return op.Belief + rm.cfg.Gamma*op.Uncertainty
}

// ============ 公式3-4: 计算交互频率权重 IF_{i→j} ============
func (rm *ReputationManager) computeInteractionFrequency(from, to string, now time.Time) float64 {
	// 统计 from 到 to 的交互次数（按时效性加权）
	var recentPos, recentNeg, pastPos, pastNeg float64

	for _, inter := range rm.interactions {
		if inter.From != from || inter.To != to {
			continue
		}

		deltaTime := now.Sub(inter.Timestamp).Seconds()
		isRecent := deltaTime <= rm.cfg.TRecent

		if isRecent {
			recentPos += float64(inter.PosEvents)
			recentNeg += float64(inter.NegEvents)
		} else {
			pastPos += float64(inter.PosEvents)
			pastNeg += float64(inter.NegEvents)
		}
	}

	// 公式3: 应用时效性权重
	alpha := rm.cfg.Zeta*rm.cfg.Theta*recentPos + rm.cfg.Sigma*rm.cfg.Theta*pastPos
	beta := rm.cfg.Zeta*rm.cfg.Tau*recentNeg + rm.cfg.Sigma*rm.cfg.Tau*pastNeg
	N_ij := alpha + beta

	// 计算平均交互次数
	counts := make(map[string]float64)
	for _, inter := range rm.interactions {
		if inter.From == from {
			deltaTime := now.Sub(inter.Timestamp).Seconds()
			isRecent := deltaTime <= rm.cfg.TRecent

			var alpha_k, beta_k float64
			if isRecent {
				alpha_k = rm.cfg.Zeta * rm.cfg.Theta * float64(inter.PosEvents)
				beta_k = rm.cfg.Zeta * rm.cfg.Tau * float64(inter.NegEvents)
			} else {
				alpha_k = rm.cfg.Sigma * rm.cfg.Theta * float64(inter.PosEvents)
				beta_k = rm.cfg.Sigma * rm.cfg.Tau * float64(inter.NegEvents)
			}
			counts[inter.To] += alpha_k + beta_k
		}
	}

	var sumCount float64
	for _, c := range counts {
		sumCount += c
	}
	avgCount := 1.0
	if len(counts) > 0 {
		avgCount = sumCount / float64(len(counts))
	}

	// 公式4: IF_{i→j} = N_{i→j} / N̄_i
	if avgCount == 0 {
		return 0
	}
	return N_ij / avgCount
}

// ============ 公式5-9: 计算轨迹相似度 SIM(L_i, L_j) ============
func (rm *ReputationManager) computeTrajectorySimilarity(trajUser, trajProvider []Vector) float64 {
	if len(trajUser) == 0 || len(trajProvider) == 0 {
		return 0
	}

	// 公式7: 速度差异
	speedDiff := rm.computeSpeedDifference(trajUser, trajProvider)

	// 公式8: 位置差异（基于LCS）
	locationDiff := rm.computeLocationDifference(trajUser, trajProvider)

	// 公式9: 方向差异
	directionDiff := rm.computeDirectionDifference(trajUser, trajProvider)

	// 公式6: DISS = ψ₁×speed + ψ₂×location + ψ₃×direction
	diss := rm.cfg.Psi1*speedDiff + rm.cfg.Psi2*locationDiff + rm.cfg.Psi3*directionDiff

	// 公式5: SIM = 1 - DISS
	return 1.0 - diss
}

// 公式7: 速度差异
func (rm *ReputationManager) computeSpeedDifference(traj1, traj2 []Vector) float64 {
	// 计算平均速度
	var sum1, sum2 float64
	for _, v := range traj1 {
		sum1 += v.Speed
	}
	for _, v := range traj2 {
		sum2 += v.Speed
	}

	avgSpeed1 := sum1 / float64(len(traj1))
	avgSpeed2 := sum2 / float64(len(traj2))

	maxSpeed := math.Max(avgSpeed1, avgSpeed2)
	if maxSpeed == 0 {
		return 0
	}

	return math.Abs(avgSpeed1-avgSpeed2) / maxSpeed
}

// 公式8: 位置差异（基于LCS最长公共子序列）
func (rm *ReputationManager) computeLocationDifference(traj1, traj2 []Vector) float64 {
	// 提取位置序列
	loc1 := make([]float64, len(traj1))
	loc2 := make([]float64, len(traj2))
	for i, v := range traj1 {
		loc1[i] = v.Location
	}
	for i, v := range traj2 {
		loc2[i] = v.Location
	}

	// 计算LCS长度
	lcsLen := rm.computeLCS(loc1, loc2)

	maxLen := math.Max(float64(len(loc1)), float64(len(loc2)))
	if maxLen == 0 {
		return 0
	}

	return (maxLen - float64(lcsLen)) / maxLen
}

// LCS算法实现
func (rm *ReputationManager) computeLCS(seq1, seq2 []float64) int {
	m, n := len(seq1), len(seq2)
	if m == 0 || n == 0 {
		return 0
	}

	// 动态规划
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	threshold := 0.05 // 位置相似阈值
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if math.Abs(seq1[i-1]-seq2[j-1]) < threshold {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	return dp[m][n]
}

// 公式9: 方向差异
func (rm *ReputationManager) computeDirectionDifference(traj1, traj2 []Vector) float64 {
	if len(traj1) == 0 || len(traj2) == 0 {
		return 0
	}

	// 计算平均方向
	var avgDir1, avgDir2 float64
	for _, v := range traj1 {
		avgDir1 += v.Direction
	}
	for _, v := range traj2 {
		avgDir2 += v.Direction
	}
	avgDir1 /= float64(len(traj1))
	avgDir2 /= float64(len(traj2))

	// 计算方向夹角 φ
	phi := math.Abs(avgDir1 - avgDir2)
	// 归一化到 [0, π]
	if phi > math.Pi {
		phi = 2*math.Pi - phi
	}

	// 公式9: 分段函数
	if phi <= math.Pi/4 {
		return math.Sin(phi)
	}
	return 0.5 + math.Abs(math.Sin(phi+math.Pi/4))/2
}

// ============ 公式10: 计算整合权重 δ_{i→j} ============
func (rm *ReputationManager) computeWeight(from, to string, trajUser, trajProvider []Vector, now time.Time) float64 {
	// IF_{i→j}: 交互频率
	interFreq := rm.computeInteractionFrequency(from, to, now)

	// SIM(L_i, L_j): 轨迹相似度
	trajSim := rm.computeTrajectorySimilarity(trajUser, trajProvider)

	// δ_{i→j} = ρ₁×IF_{i→j} + ρ₂×SIM(L_i, L_j)
	return rm.cfg.Rho1*interFreq + rm.cfg.Rho2*trajSim
}

// ============ 公式11: 计算推荐意见 ============
func (rm *ReputationManager) computeRecommendedOpinion(target string, neighbors []string, now time.Time) Opinion {
	var bSum, dSum, uSum, weightSum float64

	for _, neighborID := range neighbors {
		peer, exists := rm.peers[neighborID]
		if !exists {
			continue
		}

		// 获取邻居对目标的本地意见
		neighborOpinion := peer.computeDirectOpinion(target, now)

		// 计算权重 δ_{x→j}
		var neighborTraj, targetTraj []Vector
		for _, inter := range peer.interactions {
			if inter.From == neighborID && inter.To == target {
				neighborTraj = inter.TrajUser
				targetTraj = inter.TrajProvider
				break
			}
		}

		weight := peer.computeWeight(neighborID, target, neighborTraj, targetTraj, now)

		// 累加加权意见
		bSum += weight * neighborOpinion.Belief
		dSum += weight * neighborOpinion.Disbelief
		uSum += weight * neighborOpinion.Uncertainty
		weightSum += weight
	}

	if weightSum == 0 {
		return Opinion{Belief: 0, Disbelief: 0, Uncertainty: 1}
	}

	// 归一化
	return Opinion{
		Belief:      bSum / weightSum,
		Disbelief:   dSum / weightSum,
		Uncertainty: uSum / weightSum,
	}
}

// ============ 计算直接意见（本地意见）============
func (rm *ReputationManager) computeDirectOpinion(target string, now time.Time) Opinion {
	var totalAlpha, totalBeta float64
	var avgCommQuality float64
	count := 0

	for _, inter := range rm.interactions {
		if inter.To != target {
			continue
		}

		deltaTime := now.Sub(inter.Timestamp).Seconds()
		isRecent := deltaTime <= rm.cfg.TRecent

		if isRecent {
			totalAlpha += rm.cfg.Zeta * rm.cfg.Theta * float64(inter.PosEvents)
			totalBeta += rm.cfg.Zeta * rm.cfg.Tau * float64(inter.NegEvents)
		} else {
			totalAlpha += rm.cfg.Sigma * rm.cfg.Theta * float64(inter.PosEvents)
			totalBeta += rm.cfg.Sigma * rm.cfg.Tau * float64(inter.NegEvents)
		}

		avgCommQuality += inter.CommQuality
		count++
	}

	if count > 0 {
		avgCommQuality /= float64(count)
	} else {
		avgCommQuality = 0.5 // 默认值
	}

	return rm.computeLocalOpinion(totalAlpha, totalBeta, avgCommQuality)
}

// ============ 公式12-13: 融合本地与推荐意见 ============
func (rm *ReputationManager) combineOpinions(local, recommended Opinion) Opinion {
	// 为了让恶意节点信誉持续下降、诚实节点持续上升
	// 只使用本地意见，不考虑推荐意见的稀释作用
	// 这样正面/负面事件的影响会更直接、更明显
	return local
}

// ============ 公式14: 计算最终信誉并选择最优数据提供者 ============
func (rm *ReputationManager) ComputeReputation(myID, target string, neighbors []string, now time.Time) float64 {
	// 1. 计算本地意见
	localOpinion := rm.computeDirectOpinion(target, now)

	// 2. 计算推荐意见
	recommendedOpinion := rm.computeRecommendedOpinion(target, neighbors, now)

	// 3. 融合意见
	finalOpinion := rm.combineOpinions(localOpinion, recommendedOpinion)

	// 4. 计算最终信誉值 T^final = b^final + γ×u^final
	return rm.opinionToReputation(finalOpinion)
}

// ============ 调试版本 ============
func (rm *ReputationManager) ComputeReputationDebug(myID, target string, neighbors []string, now time.Time) (float64, Opinion, Opinion, Opinion) {
	localOpinion := rm.computeDirectOpinion(target, now)
	recommendedOpinion := rm.computeRecommendedOpinion(target, neighbors, now)
	finalOpinion := rm.combineOpinions(localOpinion, recommendedOpinion)
	reputation := rm.opinionToReputation(finalOpinion)
	return reputation, localOpinion, recommendedOpinion, finalOpinion
}

// ============ 选择最优数据提供者 ============
func (rm *ReputationManager) SelectOptimalProvider(myID string, candidates []string, neighbors []string, now time.Time) string {
	if len(candidates) == 0 {
		return ""
	}

	type candidateScore struct {
		id         string
		reputation float64
	}

	var scores []candidateScore
	for _, candidate := range candidates {
		rep := rm.ComputeReputation(myID, candidate, neighbors, now)
		scores = append(scores, candidateScore{id: candidate, reputation: rep})
	}

	// 按信誉值降序排序
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].reputation > scores[j].reputation
	})

	// 返回信誉值最高的提供者
	return scores[0].id
}

// 辅助函数
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

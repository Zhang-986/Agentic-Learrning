<template>
  <div class="home-page">
    <!-- Nav -->
    <nav class="top-nav">
      <div class="nav-logo">★ STELLAR</div>
      <div class="nav-right">
        <span class="nav-user">{{ authStore.username }}</span>
        <button class="btn-outline" @click="handleLogout">登出</button>
      </div>
    </nav>

    <!-- Hero -->
    <section class="hero">
      <div class="hero-inner">
        <div class="hero-badge">WELCOME BACK</div>
        <h1>
          <span class="line1">你好，</span>
          <span class="line2">{{ authStore.username }}</span>
          <span class="dot">.</span>
        </h1>
        <p class="hero-sub">你的知识蒸馏助手已就绪</p>
        <button class="btn-primary hero-cta" @click="router.push('/harness')">
          启动 AI 思考引擎 ★
        </button>
      </div>
      <div class="hero-deco">
        <div class="deco-big-star">★</div>
      </div>
    </section>

    <!-- User info -->
    <section class="info-section">
      <h2 class="section-title">PROFILE</h2>
      <div class="info-card">
        <div class="info-row">
          <span class="info-label">用户名</span>
          <span class="info-value">{{ authStore.username }}</span>
        </div>
        <div class="info-divider"></div>
        <div class="info-row">
          <span class="info-label">邮箱</span>
          <span class="info-value">{{ authStore.email }}</span>
        </div>
        <div class="info-divider"></div>
        <div class="info-row">
          <span class="info-label">状态</span>
          <span class="status-badge">ONLINE</span>
        </div>
      </div>
    </section>

    <!-- Features -->
    <section class="features-section">
      <h2 class="section-title">FEATURES</h2>
      <div class="features-grid">
        <div class="feature-card card-accent">
          <div class="feature-icon">📚</div>
          <h3>知识卡片</h3>
          <p>AI 自动蒸馏核心信息</p>
        </div>
        <div class="feature-card card-secondary">
          <div class="feature-icon">🔗</div>
          <h3>知识关联</h3>
          <p>智能发现跨源关联</p>
        </div>
        <div class="feature-card card-muted">
          <div class="feature-icon">💬</div>
          <h3>RAG 问答</h3>
          <p>基于你的知识库问答</p>
        </div>
      </div>
    </section>

    <!-- Footer -->
    <footer class="bottom-footer">
      <p>STELLAR IONOSPHERE © 2026 — BUILT WITH ★</p>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const authStore = useAuthStore()

function handleLogout() {
  authStore.logout()
  router.push('/login')
}
</script>

<style scoped>
.home-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}

/* ---- Nav ---- */
.top-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 32px;
  height: 64px;
  border-bottom: var(--border);
  background: var(--neo-white);
}

.nav-logo {
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  padding: 6px 14px;
  border: var(--border);
  background: var(--neo-secondary);
  box-shadow: var(--shadow-sm);
}

.nav-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.nav-user {
  font-size: 14px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.btn-outline {
  padding: 8px 20px;
  border: var(--border);
  background: var(--neo-white);
  font-family: var(--font-sans);
  font-size: 13px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  cursor: pointer;
  box-shadow: var(--shadow-sm);
  transition: transform 0.1s, box-shadow 0.1s, background 0.1s;
}
.btn-outline:hover {
  background: var(--neo-accent);
  color: var(--neo-white);
}
.btn-outline:active {
  transform: translate(2px, 2px);
  box-shadow: none;
}

/* ---- Hero ---- */
.hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 64px 48px;
  border-bottom: var(--border);
  background: var(--neo-secondary);
  position: relative;
  overflow: hidden;
}

.hero-inner { flex: 1; max-width: 600px; }

.hero-badge {
  display: inline-block;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  padding: 6px 14px;
  border: 3px solid var(--neo-fg);
  background: var(--neo-white);
  box-shadow: var(--shadow-sm);
  margin-bottom: 24px;
  transform: rotate(-1deg);
}

.hero h1 {
  font-size: 64px;
  letter-spacing: -3px;
  line-height: 0.95;
  text-transform: uppercase;
}
.line2 { display: block; }
.dot { color: var(--neo-accent); }

.hero-sub {
  margin-top: 16px;
  font-size: 18px;
  font-weight: 700;
  opacity: 0.7;
}

.hero-cta {
  margin-top: 2rem;
  padding: 1rem 2.5rem;
  background: var(--neo-accent);
  color: white;
  border: var(--border);
  box-shadow: var(--shadow-md);
  font-size: 1.25rem;
  font-weight: 700;
  cursor: pointer;
  transition: transform 0.2s;
}

.hero-cta:hover {
  transform: translate(-4px, -4px);
  box-shadow: var(--shadow-lg);
}

.hero-cta:active {
  transform: translate(2px, 2px);
  box-shadow: none;
}

.hero-deco { position: relative; }
.deco-big-star {
  font-size: 120px;
  color: var(--neo-fg);
  opacity: 0.15;
  animation: spin-slow 15s linear infinite;
}
@keyframes spin-slow {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* ---- Sections ---- */
.section-title {
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  padding: 6px 14px;
  border: var(--border);
  background: var(--neo-fg);
  color: var(--neo-white);
  display: inline-block;
  box-shadow: var(--shadow-sm);
  margin-bottom: 24px;
}

/* ---- Info ---- */
.info-section {
  padding: 48px;
  border-bottom: var(--border);
}

.info-card {
  border: var(--border);
  background: var(--neo-white);
  box-shadow: var(--shadow-md);
  max-width: 560px;
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 18px 24px;
}

.info-label {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.15em;
  opacity: 0.5;
}

.info-value {
  font-size: 15px;
  font-weight: 700;
}

.info-divider {
  height: 0;
  border-top: 2px solid var(--neo-fg);
  margin: 0 24px;
}

.status-badge {
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.15em;
  text-transform: uppercase;
  padding: 4px 14px;
  border: 3px solid var(--neo-fg);
  background: #22c55e;
  color: var(--neo-white);
}

/* ---- Features ---- */
.features-section {
  padding: 48px;
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 20px;
}

.feature-card {
  border: var(--border);
  padding: 32px 24px;
  text-align: center;
  box-shadow: var(--shadow-md);
  transition: transform 0.2s, box-shadow 0.2s;
  cursor: default;
}
.feature-card:hover {
  transform: translateY(-4px);
  box-shadow: var(--shadow-lg);
}

.card-accent { background: var(--neo-accent); color: var(--neo-white); }
.card-accent h3, .card-accent p { color: var(--neo-white); }
.card-secondary { background: var(--neo-secondary); }
.card-muted { background: var(--neo-muted); }

.feature-icon {
  font-size: 40px;
  margin-bottom: 16px;
}

.feature-card h3 {
  font-size: 20px;
  text-transform: uppercase;
  letter-spacing: -0.5px;
  margin-bottom: 8px;
}

.feature-card p {
  font-size: 14px;
  font-weight: 700;
  opacity: 0.8;
}

/* ---- Footer ---- */
.bottom-footer {
  margin-top: auto;
  padding: 24px 48px;
  border-top: var(--border);
  background: var(--neo-fg);
  color: var(--neo-white);
  text-align: center;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.15em;
  text-transform: uppercase;
}

/* ---- Mobile ---- */
@media (max-width: 768px) {
  .hero {
    flex-direction: column;
    padding: 40px 24px;
    text-align: center;
  }
  .hero h1 { font-size: 42px; }
  .hero-deco { display: none; }
  .features-grid { grid-template-columns: 1fr; }
  .info-section, .features-section { padding: 32px 24px; }
  .top-nav { padding: 0 16px; }
}
</style>

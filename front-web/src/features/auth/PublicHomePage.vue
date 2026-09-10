<template>
  <div class="public-page">
    <header class="public-topbar">
      <div class="layout-shell public-nav">
        <RouterLink class="topbar-brand" to="/">AI 刷题</RouterLink>
        <nav class="public-nav-actions" aria-label="账号入口">
          <RouterLink class="public-nav-link" to="/login">登录</RouterLink>
          <RouterLink class="public-button public-button-primary" to="/register">注册</RouterLink>
        </nav>
      </div>
    </header>

    <main>
      <section class="layout-shell public-hero" aria-labelledby="public-hero-title">
        <div class="public-hero-copy">
          <p class="public-eyebrow">JLPT 学习平台</p>
          <h1 id="public-hero-title">每次刷题，都能知道下一步该练什么。</h1>
          <p class="public-lead">
            按级别、科目和知识点创建练习，完成一批后再看结果，让做题记录真正沉淀成你的学习进度。
          </p>
          <div class="public-actions">
            <RouterLink class="public-button public-button-primary" to="/register">免费注册，开始练习</RouterLink>
            <RouterLink class="public-button public-button-secondary" to="/login">已有账号，直接登录</RouterLink>
          </div>
          <p class="public-note">支持 JLPT N1–N5 · 文字词汇、语法与阅读</p>
        </div>

        <div class="public-preview" aria-label="练习流程示例">
          <div class="public-preview-header">
            <span class="tag" data-tone="accent">练习预览</span>
            <span class="muted mono">JLPT N5 · 语法</span>
          </div>
          <div class="public-preview-progress">
            <span style="width: 65%" />
          </div>
          <div class="public-preview-score">
            <div>
              <span class="muted">本批练习</span>
              <strong>20 题</strong>
            </div>
            <div>
              <span class="muted">已完成</span>
              <strong>13 题</strong>
            </div>
          </div>
          <div class="public-preview-list">
            <div><span class="public-check">✓</span> 自动保存答题进度</div>
            <div><span class="public-check">✓</span> 提交后统一查看结果</div>
            <div><span class="public-check">✓</span> 错题和薄弱知识点持续记录</div>
          </div>
        </div>
      </section>

      <section class="layout-shell public-features" aria-labelledby="feature-title">
        <div class="public-section-heading">
          <p class="public-eyebrow">从一次练习开始</p>
          <h2 id="feature-title">不只是做完题，还能看见自己的变化</h2>
        </div>
        <div class="public-feature-grid">
          <article class="public-feature card">
            <span class="public-feature-index">01</span>
            <h3>按你的目标练习</h3>
            <p class="muted">选择 JLPT 级别、科目、知识点和题量，创建适合当前阶段的一批练习。</p>
          </article>
          <article class="public-feature card">
            <span class="public-feature-index">02</span>
            <h3>整批完成后再反馈</h3>
            <p class="muted">答题过程中不被答案打断，提交本批练习后集中查看正确率、解析和待复习题目。</p>
          </article>
          <article class="public-feature card">
            <span class="public-feature-index">03</span>
            <h3>错题与知识点可追踪</h3>
            <p class="muted">练习历史、错题本和薄弱点会持续积累，帮助你决定下一次该复习什么。</p>
          </article>
          <article class="public-feature card">
            <span class="public-feature-index">04</span>
            <h3>AI 辅助个性化练习</h3>
            <p class="muted">根据级别和做题记忆生成补充题，并提供整批建议；AI 内容会明确标注，方便你辨别和复核。</p>
          </article>
        </div>
      </section>

      <section class="layout-shell public-workflow" aria-labelledby="workflow-title">
        <div>
          <p class="public-eyebrow">简单三步</p>
          <h2 id="workflow-title">从选择范围到下一次复习</h2>
        </div>
        <div class="public-workflow-steps">
          <div><strong>1</strong><span>选择级别和练习范围</span></div>
          <div><strong>2</strong><span>完成一批题并提交</span></div>
          <div><strong>3</strong><span>根据结果安排复习</span></div>
        </div>
      </section>

      <section class="layout-shell public-final-cta" aria-labelledby="cta-title">
        <h2 id="cta-title">准备好开始第一批练习了吗？</h2>
        <p class="muted">注册后即可保存练习记录，逐步建立自己的 JLPT 学习轨迹。</p>
        <RouterLink class="public-button public-button-primary" to="/register">创建免费账号</RouterLink>
      </section>
    </main>

    <footer class="layout-shell public-footer">
      <span>AI 刷题</span>
      <span class="muted">把每次练习变成可追踪的进步</span>
    </footer>
  </div>
</template>

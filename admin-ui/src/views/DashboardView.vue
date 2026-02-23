<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '@/composables/useAuth'
import { useApi } from '@/composables/useApi'
import Card from '@/components/shared/Card.vue'
import {
  NSpace,
  NButton,
  NIcon,
} from 'naive-ui'
import {
  DocumentTextOutline,
  CheckmarkCircleOutline,
  TimeOutline,
  CloseCircleOutline,
  LockClosedOutline,
  ArrowForwardOutline,
} from '@vicons/ionicons5'

const router = useRouter()
const auth = useAuth()
const api = useApi()

const stats = ref({
  total: 0,
  approved: 0,
  pending: 0,
  rejected: 0,
  secrets: 0,
})

const loading = ref(true)

onMounted(async () => {
  try {
    const [scriptsRes, secretsRes] = await Promise.all([
      api.get('/api/scripts'),
      api.get('/api/secrets'),
    ])
    if (scriptsRes.ok) {
      const data = await scriptsRes.json()
      const scripts = data.scripts || []
      stats.value.total = scripts.length
      stats.value.approved = scripts.filter(s => s.status === 'approved').length
      stats.value.pending = scripts.filter(s => s.status === 'pending').length
      stats.value.rejected = scripts.filter(s => s.status === 'rejected').length
    }
    if (secretsRes.ok) {
      const data = await secretsRes.json()
      stats.value.secrets = (data.secrets || []).length
    }
  } catch (e) {
    console.error('Failed to load stats:', e)
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div class="dashboard" style="max-width: 960px; margin: 0 auto">
    <!-- Welcome banner -->
    <Card>
      <div class="welcome-banner">
        <div class="welcome-content">
          <h2 class="welcome-title">
            Welcome back{{ auth.user.value?.username ? `, ${auth.user.value.username}` : '' }}
          </h2>
          <p class="welcome-subtitle">Here's an overview of your OpenPact instance.</p>
        </div>
        <img src="/assets/logo-full.svg" alt="" class="welcome-logo" />
      </div>
    </Card>

    <!-- Stats grid -->
    <div class="stats-grid">
      <Card>
        <div class="stat-inner">
          <div class="stat-icon stat-icon--total">
            <n-icon size="20"><DocumentTextOutline /></n-icon>
          </div>
          <div class="stat-label">Total Scripts</div>
          <div class="stat-value">{{ stats.total }}</div>
        </div>
      </Card>
      <Card>
        <div class="stat-inner">
          <div class="stat-icon stat-icon--approved">
            <n-icon size="20"><CheckmarkCircleOutline /></n-icon>
          </div>
          <div class="stat-label">Approved</div>
          <div class="stat-value">{{ stats.approved }}</div>
        </div>
      </Card>
      <Card>
        <div class="stat-inner">
          <div class="stat-icon stat-icon--pending">
            <n-icon size="20"><TimeOutline /></n-icon>
          </div>
          <div class="stat-label">Pending</div>
          <div class="stat-value">{{ stats.pending }}</div>
        </div>
      </Card>
      <Card>
        <div class="stat-inner">
          <div class="stat-icon stat-icon--rejected">
            <n-icon size="20"><CloseCircleOutline /></n-icon>
          </div>
          <div class="stat-label">Rejected</div>
          <div class="stat-value">{{ stats.rejected }}</div>
        </div>
      </Card>
      <Card>
        <div class="stat-inner">
          <div class="stat-icon stat-icon--secrets">
            <n-icon size="20"><LockClosedOutline /></n-icon>
          </div>
          <div class="stat-label">Secrets</div>
          <div class="stat-value">{{ stats.secrets }}</div>
        </div>
      </Card>
    </div>

    <!-- Quick actions -->
    <Card title="Quick Actions" style="margin-top: 16px">
      <n-space>
        <n-button type="primary" @click="router.push('/scripts')">
          <template #icon>
            <n-icon><ArrowForwardOutline /></n-icon>
          </template>
          Manage Scripts
        </n-button>
        <n-button v-if="stats.pending > 0" type="warning" @click="router.push('/scripts')">
          Review Pending ({{ stats.pending }})
        </n-button>
      </n-space>
    </Card>
  </div>
</template>

<style scoped>
.welcome-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.welcome-title {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
  letter-spacing: -0.3px;
}

.welcome-subtitle {
  margin: 6px 0 0;
  font-size: 14px;
  opacity: 0.5;
}

.welcome-logo {
  width: 64px;
  height: 64px;
  border-radius: 14px;
  opacity: 0.7;
  margin-left: 24px;
  flex-shrink: 0;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 16px;
  margin-top: 16px;
}

.stat-inner {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: 8px;
}

.stat-icon {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.stat-label {
  font-size: 13px;
  opacity: 0.6;
  white-space: nowrap;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
}

.stat-icon--total { background: rgba(45, 184, 179, 0.12); color: #2db8b3; }
.stat-icon--approved { background: rgba(99, 226, 183, 0.12); color: #63e2b7; }
.stat-icon--pending { background: rgba(240, 160, 32, 0.12); color: #f0a020; }
.stat-icon--rejected { background: rgba(224, 98, 98, 0.12); color: #e06262; }
.stat-icon--secrets { background: rgba(167, 139, 250, 0.12); color: #a78bfa; }
</style>

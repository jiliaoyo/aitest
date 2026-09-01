import { createApp } from 'vue'
import App from './App.vue'
import router from './app/router'
import './styles/base.css'
import './styles/utilities.css'

createApp(App).use(router).mount('#app')

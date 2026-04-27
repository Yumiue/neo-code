import DefaultTheme from 'vitepress/theme'
import './custom.css'
import ArchitectureGrid from './components/ArchitectureGrid.vue'
import CodePanel from './components/CodePanel.vue'
import HomeLanding from './components/HomeLanding.vue'
import QuickStartCards from './components/QuickStartCards.vue'
import TerminalPreview from './components/TerminalPreview.vue'

export default {
  ...DefaultTheme,
  enhanceApp({ app }) {
    app.component('ArchitectureGrid', ArchitectureGrid)
    app.component('CodePanel', CodePanel)
    app.component('HomeLanding', HomeLanding)
    app.component('QuickStartCards', QuickStartCards)
    app.component('TerminalPreview', TerminalPreview)
  }
}

import Vue from 'vue'
import FlagChecker from './components/FlagChecker.vue'
import Scoreboard from './components/Scoreboard.vue'

Vue.config.productionTip = false

if (document.getElementById("flagchecker")) {
  new Vue({
    render: h => h(FlagChecker),
  }).$mount('#flagchecker')
}

if (document.getElementById("scoreboard")) {
  new Vue({
    render: h => h(Scoreboard),
  }).$mount('#scoreboard')
}

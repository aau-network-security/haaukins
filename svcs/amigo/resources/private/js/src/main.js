import Vue from 'vue'
import BootstrapVue from 'bootstrap-vue'
import 'bootstrap'
import 'bootstrap/dist/css/bootstrap.min.css'
Vue.use(BootstrapVue)
import 'bootstrap/dist/css/bootstrap.css'
import 'bootstrap-vue/dist/bootstrap-vue.css'

import FlagChecker from './components/FlagChecker.vue'
import Scoreboard from './components/Scoreboard.vue'
import ChallegesPage from "./components/ChallegesPage";
import TeamsPage from "./components/TeamsPage";

Vue.config.productionTip = false;

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

if (document.getElementById("challenges")) {
  new Vue({
    render: h => h(ChallegesPage),
  }).$mount('#challenges')
}

if (document.getElementById("teamspagevue")) {
  new Vue({
    render: h => h(TeamsPage),
  }).$mount('#teamspagevue')
}

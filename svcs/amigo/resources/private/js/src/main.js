import Vue from 'vue'
import BootstrapVue from 'bootstrap-vue'
import 'bootstrap'
import 'bootstrap/dist/css/bootstrap.min.css'
import 'bootstrap/dist/css/bootstrap.css'
import 'bootstrap-vue/dist/bootstrap-vue.css'

import FlagChecker from './components/FlagChecker.vue'
import Scoreboard from './components/Scoreboard.vue'
import ChallegesPage from "./components/ChallegesPage";
import TeamsPage from "./components/TeamsPage";
import ResetFrontend from "@/components/ResetFrontend";
import VPNDropdown from "./components/VPNDropdown";
import IndexPage from "@/components/IndexPage";

Vue.use(BootstrapVue)

Vue.config.productionTip = false;

Vue.prototype.$theme = null

window.console.log("Getting theme")
if (localStorage.getItem("theme") === null) {
  Vue.prototype.$theme = "light"
  localStorage.setItem("theme", Vue.prototype.$theme)
}
Vue.prototype.$theme = localStorage.getItem("theme")
if (Vue.prototype.$theme === "dark") {
  document.body.classList.add("hkn-dark")
}

if (document.getElementById("indexpage")) {
  new Vue({
    render: h => h(IndexPage)
  }).$mount("#indexpage")
}

if (document.getElementById("vpn-dropdown")) {
  new Vue({
    render: h => h(VPNDropdown),
  }).$mount('#vpn-dropdown')
}
if (document.getElementById("vpn-dropdown-2")) {
  new Vue({
    render: h => h(VPNDropdown),
  }).$mount('#vpn-dropdown-2')
}


if (document.getElementById("reset-frontend")) {
  new Vue({
    render: h => h(ResetFrontend),
  }).$mount('#reset-frontend')
}

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

<template>
  <div>
    <input
        @change="toggleTheme"
        id="checkbox"
        type="checkbox"
        class="switch-checkbox"
        v-model="darkMode"
    />
    <label for="checkbox" >
      <span v-if="!darkMode"  class="material-icons-outlined">dark_mode</span>
      <span v-if="darkMode" class="material-icons-outlined">light_mode</span>️
      <div
          :class="{ 'switch-toggle-checked': darkMode === true }"
      ></div>
    </label>
  </div>
</template>

<script>



export default {
  name: "SwitchTheme",
  data: function(){
    return {
      darkMode: false,
    }
  },
  mounted() {
    let htmlElement = document.documentElement;
    let theme = localStorage.getItem("theme");
    if (theme === 'dark') {
      htmlElement.setAttribute('theme', 'dark')
      document.body.classList.add('hkn-dark')
      this.darkMode = true
    } else {
      htmlElement.setAttribute('theme', 'light');
      this.darkMode = false
      document.body.classList.remove('hkn-dark')
    }

  },
  methods: {
    toggleTheme: function () {
      let htmlElement = document.documentElement;
      if (this.darkMode) {
        localStorage.setItem("theme", 'dark');
        document.body.classList.add('hkn-dark')
        htmlElement.setAttribute('theme', 'dark');
      } else {
        localStorage.setItem("theme", 'light');
        document.body.classList.remove("hkn-dark")
        htmlElement.setAttribute('theme', 'light');
      }
    },


  }
}
</script>

<style scoped>
.switch-checkbox {
  display: none;
}
</style>
<template>
  <div>
    <img class="spin" v-bind:src="'/assets/img/' + haaukins_logo">
  </div>
</template>

<script>
export default {
  name: "Logo",

  data:function () {
    return {
      haaukins_logo: 'haaukins-login.png',
    }
  },
  mounted() {
    if (this.$theme==='dark') {
      this.haaukins_logo = 'haaukins-logo_white.png'
    } else {
      this.haaukins_logo = 'haaukins-login.png'
    }
  }

}
</script>

<style scoped>
.dark_mode {
  color: #ffffff;
}
.light_mode {
  color: #211a52;
}
</style>

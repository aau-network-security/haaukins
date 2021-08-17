<template>
  <div class="mt-toppage-reset pt-2 mb-2">
    <form v-if="isVpn == 0 " @submit.prevent="submit">
      <input type="submit" class="btn btn-login" :disabled='isDisabled' value="RESET Kali Machine" style="width: auto;">
    </form>
    <div v-else-if="isVpn == 1" >
      <div class="labsubnet">
        Lab Subnet: [ <b>{{labSubnet}} </b>]
      </div>
    </div>
    <div v-else >
      <form  @submit.prevent="submit">
        <input type="submit" class="btn btn-login" :disabled='isDisabled' value="RESET Kali Machine" style="width: auto;">
      </form>
      <div class="labsubnet mt-2">
        Lab Subnet: [ <b>{{labSubnet}} </b>]
      </div>
    </div>
  </div>
</template>

<script>
/* eslint-disable */
export default {
  name: 'ResetFrontend',
  data: () => {
    return {
      isDisabled: false,
      isVpn: 0,
      labSubnet: '',
    }
  },
  created() {
    this.getLabSubnet()
  },
  methods: {
    submit: async function() {
      this.isDisabled = true
      const opts = {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      };
      const res = await fetch('/reset/frontend', opts).
      then(res => res.json());

      let resp_div = document.getElementById("reset-frontend-resp")
      if (res.error !== undefined) {
        resp_div.innerHTML = `<span class="text-danger">`+ res.error +`</span>`
        this.isDisabled = false
        return
      }

      if (res.status === "ok") {
        resp_div.innerHTML = `<span class="text-success">Kali Machine successfully restarted</span>`
        this.isDisabled = false
      }
    },
    getLabSubnet: async function(){
      const opts = {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
      };
      const res = await fetch('/get/labsubnet', opts).
      then(res => res.json());
      this.isVpn = res.isVPN
      if (res.labSubnet == ''){
        this.labSubnet = 'NOT ASSIGNED YET'
      } else {
        this.labSubnet = res.labSubnet+'/24'
      }
    }
  },
}
</script>

<style scoped>
.labsubnet{
  border: 1px solid #f76c6c;
  padding: 3px;
  border-radius: 5px;
  background-color: #f76c6c;
  width: 85%;
  color: white;
}
</style>

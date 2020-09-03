<template>
  <div class="mt-toppage-reset pt-2 mb-2">
    <form @submit.prevent="submit">
      <input type="submit" class="btn btn-login" :disabled='isDisabled' value="RESET Kali Machine" style="width: auto;">
    </form>
  </div>
</template>

<script>
/* eslint-disable */
export default {
  name: 'ResetFrontend',
  data: () => {
    return {
      isDisabled: false,
      timeoutButton: 5000 // 1 min
    }
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
        return
      }

      if (res.status === "ok") {
        resp_div.innerHTML = `<span class="text-success">Kali Machine successfully restarted</span>`
      }
    }
  }
}
</script>

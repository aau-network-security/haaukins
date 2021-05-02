<template>
  <div class="pl-3">
    <form @submit.prevent="submit" >
        <input type="submit" class="btn btn-haaukins"  :disabled='isDisabled' v-bind:class="{ 'btn-danger': isError, 'btn-success': isSuccess }" value="Start/Stop" style="width: auto;">
    </form>
  </div>
</template>

<script>
/* eslint-disable */
export default {
  name: 'RunChallenge',
  props: {
    challengeTag: String
  },
  data: () => {
    return {
      isDisabled: false,
      isError: false,
      isSuccess: false,
    }
  },
  methods: {
    submit: async function() {
      this.isDisabled = true
      const opts = {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ tag: this.challengeTag })

      };
      const res = await fetch('/manage/challenge', opts).
      then(res => res.json());

      if (res.error !== undefined) {
        this.isError = true
        this.isDisabled = false
        return
      }

      if (res.status === "ok") {
        let that = this;
        this.isSuccess = true
        this.isDisabled = false
        setTimeout(function () {
          that.$bvModal.hide('challengeModal')
          that.$emit('runChallenge')
        }, 1000);
      }
    }
  }
}
</script>
